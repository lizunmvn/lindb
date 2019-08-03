package memdb

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/lindb/lindb/kv"
	"github.com/lindb/lindb/pkg/interval"
	"github.com/lindb/lindb/pkg/logger"
	"github.com/lindb/lindb/pkg/timeutil"
	pb "github.com/lindb/lindb/rpc/proto/field"
	"github.com/lindb/lindb/sql/stmt"
	"github.com/lindb/lindb/tsdb/index"
	"github.com/lindb/lindb/tsdb/metrictbl"
	"github.com/lindb/lindb/tsdb/series"

	"github.com/segmentio/fasthash/fnv1a"
)

var memDBLogger = logger.GetLogger("memdb")

//go:generate mockgen -source ./database.go -destination=./database_mock.go -package memdb

// MemoryDatabase is a database-like concept of Shard as memTable in cassandra.
type MemoryDatabase interface {
	// WithMaxTagsLimit spawn a goroutine to receives limitation from this channel
	// The producer shall send the config periodically
	// key: metric-name, value: max-limit
	WithMaxTagsLimit(<-chan map[string]uint32)
	// Write writes metrics to the memory-database,
	// return error on exceeding max count of tagsIdentifier or writing failure
	Write(metric *pb.Metric) error
	// ResetMetricStore reassigns a new version to metricStore
	// This method provides the ability to reset the tsStore in memory for skipping the tsID-limitation
	ResetMetricStore(metricName string) error
	// CountMetrics returns the metrics-count of the memory-database
	CountMetrics() int
	// CountTags returns the tags-count of the metricName, return -1 if not exist
	CountTags(metricName string) int
	// Families returns the families in memory which has not been flushed yet
	Families() []int64
	// FlushFamilyTo flushes the corresponded family data to builder.
	// Close is not in the flushing process.
	FlushFamilyTo(flusher kv.Flusher, familyTime int64) error
	// SeriesIDsFilter contains the methods for filtering seriesIDs from memDB
	index.SeriesIDsFilter
}

// mStoresBucket is a simple rwMutex locked map of metricStore.
type mStoresBucket struct {
	rwLock      sync.RWMutex
	familyTimes map[int64]struct{}    // familyTimes unions all mStores
	hash2MStore map[uint64]mStoreINTF // key: FNV64a(metric-name)
}

// addFamilyTime adds a family-time to the map
func (bkt *mStoresBucket) addFamilyTime(familyTime int64) {
	bkt.rwLock.RLock()
	bkt.familyTimes[familyTime] = struct{}{}
	bkt.rwLock.RUnlock()
}

// unionFamilyTimesTo unions the internal familyTime to the input map.
func (bkt *mStoresBucket) unionFamilyTimesTo(segments map[int64]struct{}) {
	bkt.rwLock.RLock()
	for familyTime := range bkt.familyTimes {
		segments[familyTime] = struct{}{}
	}
	bkt.rwLock.RUnlock()
}

// unionFamilyTimesFrom unions the familyTimes map to the internal map.
func (bkt *mStoresBucket) unionFamilyTimesFrom(familyTimes map[int64]struct{}) {
	bkt.rwLock.RLock()
	for familyTime := range familyTimes {
		bkt.familyTimes[familyTime] = struct{}{}
	}
	bkt.rwLock.RUnlock()
}

// allMetricStores returns a clone of metric-hashes and pointer of mStores in bucket.
func (bkt *mStoresBucket) allMetricStores() (metricHashes []uint64, stores []mStoreINTF) {
	bkt.rwLock.RLock()
	length := len(bkt.hash2MStore)
	metricHashes = make([]uint64, length)
	stores = make([]mStoreINTF, length)
	idx := 0
	for metricHash, mStore := range bkt.hash2MStore {
		// delete tag of tStore which has not been used for a while
		metricHashes[idx] = metricHash
		stores[idx] = mStore
		idx++
	}
	bkt.rwLock.RUnlock()
	return
}

// memoryDatabase implements MemoryDatabase.
type memoryDatabase struct {
	timeWindow    int                                    // rollup window of memory-database
	interval      int64                                  // time interval of rollup
	intervalType  interval.Type                          // month, day, hour
	intervalCalc  interval.Calculator                    // helper function for calculating interval
	blockStore    *blockStore                            // reusable pool
	ctx           context.Context                        // used for exiting goroutines
	evictNotifier chan struct{}                          // notifying evictor to evict
	once4Syncer   sync.Once                              // once for tags-limitation syncer
	metricID2Hash sync.Map                               // key: metric-id(uint32), value: hash(uint64)
	mStoresList   [shardingCountOfMStores]*mStoresBucket // metric-name -> *metricStore
	generator     index.IDGenerator                      // the generator for generating ID of metric, field
}

// NewMemoryDatabase returns a new MemoryDatabase.
func NewMemoryDatabase(ctx context.Context, timeWindow int,
	intervalValue int64, intervalType interval.Type) (MemoryDatabase, error) {

	timeCalc, err := interval.GetCalculator(intervalType)
	if err != nil {
		return nil, err
	}
	md := memoryDatabase{
		timeWindow:    timeWindow,
		interval:      intervalValue,
		intervalType:  intervalType,
		intervalCalc:  timeCalc,
		blockStore:    newBlockStore(timeWindow),
		ctx:           ctx,
		evictNotifier: make(chan struct{})}
	for i := range md.mStoresList {
		md.mStoresList[i] = &mStoresBucket{
			familyTimes: make(map[int64]struct{}),
			hash2MStore: make(map[uint64]mStoreINTF)}
	}
	go md.evictor(ctx)
	return &md, nil
}

// getBucket returns the mStoresBucket by metric-hash.
func (md *memoryDatabase) getBucket(metricHash uint64) *mStoresBucket {
	return md.mStoresList[shardingCountMask&metricHash]
}

// getMStore returns the mStore by metric-name.
func (md *memoryDatabase) getMStore(metricName string) (mStore mStoreINTF, ok bool) {
	return md.getMStoreByMetricHash(fnv1a.HashString64(metricName))
}

// getMStoreByMetricHash returns the mStore by metric-hash.
func (md *memoryDatabase) getMStoreByMetricHash(hash uint64) (mStore mStoreINTF, ok bool) {
	bkt := md.getBucket(hash)
	bkt.rwLock.RLock()
	mStore, ok = bkt.hash2MStore[hash]
	bkt.rwLock.RUnlock()
	return
}

// getMStoreByMetricID returns the mStore by metricID.
func (md *memoryDatabase) getMStoreByMetricID(metricID uint32) (mStore mStoreINTF, ok bool) {
	item, ok := md.metricID2Hash.Load(metricID)
	if !ok {
		return nil, false
	}
	return md.getMStoreByMetricHash(item.(uint64))
}

// getOrCreateMStore returns the mStore by metricHash.
func (md *memoryDatabase) getOrCreateMStore(metricName string, hash uint64) mStoreINTF {
	bucket := md.getBucket(hash)

	var mStore mStoreINTF
	mStore, ok := md.getMStoreByMetricHash(hash)
	if !ok {
		metricID := md.generator.GenMetricID(metricName)
		newMStore := newMetricStore(metricID)

		bucket.rwLock.Lock()
		mStore, ok = bucket.hash2MStore[hash]
		if !ok {
			bucket.hash2MStore[hash] = newMStore
			md.metricID2Hash.Store(metricID, hash)
		}
		bucket.rwLock.Unlock()
	}
	return mStore
}

// WithMaxTagsLimit syncs the limitation for different metrics.
func (md *memoryDatabase) WithMaxTagsLimit(limitationCh <-chan map[string]uint32) {
	md.once4Syncer.Do(func() {
		go func() {
			for {
				select {
				case <-md.ctx.Done():
					return
				case limitations, ok := <-limitationCh:
					if !ok {
						return
					}
					if limitations == nil {
						continue
					}
					md.setLimitations(limitations)
				}
			}
		}()
	})
}

// setLimitations set max-count limitation of tagID.
func (md *memoryDatabase) setLimitations(limitations map[string]uint32) {
	for metricName, limit := range limitations {
		mStore, ok := md.getMStore(metricName)
		if !ok {
			continue
		}
		mStore.setMaxTagsLimit(limit)
	}
}

// writeContext holds the context for writing
type writeContext struct {
	blockStore   *blockStore
	generator    index.IDGenerator
	metricID     uint32
	familyTime   int64
	slotIndex    int
	timeInterval int64
}

// Write writes metric-point to database.
func (md *memoryDatabase) Write(metric *pb.Metric) (err error) {
	timestamp := metric.Timestamp
	// calculate family start time and slot index
	segmentTime := md.intervalCalc.CalSegmentTime(timestamp)                      // day
	family := md.intervalCalc.CalFamily(timestamp, segmentTime)                   // hours
	familyStartTime := md.intervalCalc.CalFamilyStartTime(segmentTime, family)    // family timestamp
	slotIndex := md.intervalCalc.CalSlot(timestamp, familyStartTime, md.interval) // slot offset of family

	hash := fnv1a.HashString64(metric.Name)
	mStore := md.getOrCreateMStore(metric.Name, hash)
	// todo: @codingcrush, pass it as milliseconds
	err = mStore.write(metric, writeContext{
		metricID:     mStore.getMetricID(),
		blockStore:   md.blockStore,
		generator:    md.generator,
		familyTime:   familyStartTime,
		slotIndex:    slotIndex,
		timeInterval: md.interval})
	if err == nil {
		bkt := md.getBucket(hash)
		bkt.addFamilyTime(familyStartTime)
	}
	return
}

// evictor do evict periodically.
func (md *memoryDatabase) evictor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-md.evictNotifier:
			for i := 0; i < shardingCountOfMStores; i++ {
				md.evict(md.mStoresList[i&shardingCountMask])
			}
		}
	}
}

// evict evicts tsStore of mStore concurrently,
// and delete metricStore whose timeSeriesMap is empty.
func (md *memoryDatabase) evict(bucket *mStoresBucket) {
	// get all allMStores
	metricHashes, allMStores := bucket.allMetricStores()

	for idx, mStore := range allMStores {
		// delete tag of tStore which has not been used for a while
		mStore.evict()
		// delete mStore whose tags is empty now.
		if mStore.isEmpty() {
			bucket.rwLock.Lock()
			if mStore.isEmpty() {
				delete(bucket.hash2MStore, metricHashes[idx])
				md.metricID2Hash.Delete(mStore.getMetricID())
			}
			bucket.rwLock.Unlock()
		}
	}
}

// ResetMetricStore assigns a new version to the specified metric.
func (md *memoryDatabase) ResetMetricStore(metricName string) error {
	mStore, ok := md.getMStore(metricName)
	if !ok {
		return fmt.Errorf("metric: %s doesn't exist", metricName)
	}
	return mStore.resetVersion()
}

// CountMetrics returns count of metrics in all buckets.
func (md *memoryDatabase) CountMetrics() int {
	var counter = 0
	for bucketIndex := 0; bucketIndex < shardingCountOfMStores; bucketIndex++ {
		md.mStoresList[bucketIndex].rwLock.RLock()
		counter += len(md.mStoresList[bucketIndex].hash2MStore)
		md.mStoresList[bucketIndex].rwLock.RUnlock()
	}
	return counter
}

// CountTags returns count of tags of a specified metricName, return -1 when metric not exist.
func (md *memoryDatabase) CountTags(metricName string) int {
	mStore, ok := md.getMStore(metricName)
	if !ok {
		return -1
	}
	return mStore.getTagsCount()
}

// Families returns the families in memory which has not been flushed yet.
func (md *memoryDatabase) Families() []int64 {
	families := make(map[int64]struct{})
	for bucketIndex := 0; bucketIndex < shardingCountOfMStores; bucketIndex++ {
		bkt := md.mStoresList[bucketIndex]
		bkt.unionFamilyTimesTo(families)
	}
	var list []int64
	for familyTime := range families {
		list = append(list, familyTime)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i] < list[j]
	})
	return list
}

// FlushFamilyTo flushes all data related to the family from metric-stores to builder,
// this method must be called before the cancellation.
func (md *memoryDatabase) FlushFamilyTo(flusher kv.Flusher, familyTime int64) error {
	tableFlusher := metrictbl.NewTableFlusher(flusher, md.interval)
	return md.flushFamilyTo(tableFlusher, familyTime)
}

// flushContext holds the context for flushing
type flushContext struct {
	metricID     uint32
	familyTime   int64
	timeInterval int64
}

// flushFamilyTo is the real flush method, used for mock-test
func (md *memoryDatabase) flushFamilyTo(tableFlusher metrictbl.TableFlusher, familyTime int64) error {
	defer func() {
		// non-block notifying evictor
		select {
		case md.evictNotifier <- struct{}{}:
		default:
			memDBLogger.Warn("flusher is working, concurrently flushing is not allowed")
		}
	}()
	// union the familyTimes back whatever error is raised
	var oldFamilyTimesList []map[int64]struct{}
	defer func() {
		for bucketIndex, familyTimes := range oldFamilyTimesList {
			bkt := md.mStoresList[bucketIndex]
			bkt.unionFamilyTimesFrom(familyTimes)
		}
	}()

	var err error
	for bucketIndex := 0; bucketIndex < shardingCountOfMStores; bucketIndex++ {
		bkt := md.mStoresList[bucketIndex]
		bkt.rwLock.Lock()
		oldFamilyTimesList = append(oldFamilyTimesList, bkt.familyTimes)
		bkt.familyTimes = make(map[int64]struct{})
		bkt.rwLock.Unlock()

		_, allMetricStores := bkt.allMetricStores()
		for _, mStore := range allMetricStores {
			if err = mStore.flushMetricsTo(tableFlusher, flushContext{
				metricID:     mStore.getMetricID(),
				familyTime:   familyTime,
				timeInterval: md.interval,
			}); err != nil {
				return err
			}
		}
		// remove familyTime from oldFamilyTimesList
		delete(oldFamilyTimesList[bucketIndex], familyTime)
	}
	return nil
}

// FindSeriesIDsByExpr finds series ids by tag filter expr for metric id from mStore.
func (md *memoryDatabase) FindSeriesIDsByExpr(metricID uint32, expr stmt.TagFilter,
	timeRange timeutil.TimeRange) (*series.MultiVerSeriesIDSet, error) {

	mStore, ok := md.getMStoreByMetricID(metricID)
	if !ok {
		return nil, fmt.Errorf("metricID: %d not found", metricID)
	}
	return mStore.findSeriesIDsByExpr(expr, timeRange)
}

// GetSeriesIDsForTag get series ids for spec metric's tag key from mStore.
func (md *memoryDatabase) GetSeriesIDsForTag(metricID uint32, tagKey string,
	timeRange timeutil.TimeRange) (*series.MultiVerSeriesIDSet, error) {

	mStore, ok := md.getMStoreByMetricID(metricID)
	if !ok {
		return nil, fmt.Errorf("metricID: %d not found", metricID)
	}
	return mStore.getSeriesIDsForTag(tagKey, timeRange)
}
