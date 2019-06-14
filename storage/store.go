package storage

import (
	"path/filepath"
	"sync"

	"github.com/eleme/lindb/pkg/logger"
	"github.com/eleme/lindb/pkg/util"
	meta "github.com/eleme/lindb/storage/version"

	"go.uber.org/zap"
)

type Store struct {
	name     string
	lock     *Lock
	option   StoreOption
	versions *meta.VersionSet
	families map[string]*Family
	mutex    sync.Mutex
	logger   *zap.Logger
}

func NewStore(name string, option StoreOption) (*Store, error) {
	if err := util.MkDirIfNotExist(option.Path); err != nil {
		return nil, err
	}

	// file lock, only allow open by a instance
	lock := NewLock(filepath.Join(option.Path, meta.Lock))
	err := lock.Lock()
	if err != nil {
		return nil, err
	}

	log := logger.GetLogger()

	// unlock file lock if error
	defer func() {
		if err != nil {
			e := lock.Unlock()
			if e != nil {
				log.Error("unlock file error:", zap.Error(e))
			}
		}
	}()

	store := &Store{
		name:     name,
		option:   option,
		lock:     lock,
		families: make(map[string]*Family),
		logger:   log,
	}

	// init and recover version set
	vs := meta.NewVersionSet(store.option.Path)
	err = vs.Recover()
	if err != nil {
		return nil, err
	}

	store.versions = vs
	return store, nil
}

func (s *Store) CreateFamily(familyName string, option FamilyOption) (*Family, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var family, ok = s.families[familyName]
	if !ok {
		familyPath := filepath.Join(s.option.Path, familyName)

		var err error
		if !util.Exist(familyPath) {
			// create new family
			family, err = NewFamily(s, familyName, option)
		} else {
			// open exist family
			family, err = OpenFamily(s, familyName)
		}

		if err != nil {
			return nil, err
		}
		s.families[familyName] = family
	}

	return family, nil
}

func (s *Store) GetFamily(familyName string) (*Family, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	family, ok := s.families[familyName]
	return family, ok
}

func (s *Store) Close() error {
	return s.lock.Unlock()
}
