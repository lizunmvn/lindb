package service

import (
	"testing"

	"github.com/lindb/lindb/mock"
	"github.com/lindb/lindb/models"
	"github.com/lindb/lindb/pkg/state"

	"gopkg.in/check.v1"
)

type testStorageStateSRVSuite struct {
	mock.RepoTestSuite
}

func TestStorageStateSRV(t *testing.T) {
	check.Suite(&testStorageStateSRVSuite{})
	check.TestingT(t)
}

func (ts *testStorageStateSRVSuite) TestStorageState(c *check.C) {
	repo, _ := state.NewRepo(state.Config{
		Namespace: "/test/storage/state",
		Endpoints: ts.Cluster.Endpoints,
	})
	storageState := models.NewStorageState()
	storageState.Name = "LinDB_Storage"
	storageState.AddActiveNode(&models.ActiveNode{Node: models.Node{IP: "1.1.1.1", Port: 9000}})

	srv := NewStorageStateService(repo)

	err := srv.Save("Test_LinDB", storageState)
	if err != nil {
		c.Fatal(err)
	}

	storageState1, _ := srv.Get("Test_LinDB")

	c.Assert(storageState, check.DeepEquals, storageState1)

	_, err = srv.Get("Test_LinDB_Not_Exist")
	c.Assert(state.ErrNotExist, check.Equals, err)

	_ = repo.Close()

	// test error
	err = srv.Save("Test_LinDB", storageState)
	c.Assert(err, check.NotNil)
	storageState1, err = srv.Get("Test_LinDB")
	c.Assert(storageState1, check.IsNil)
	c.Assert(err, check.NotNil)
}
