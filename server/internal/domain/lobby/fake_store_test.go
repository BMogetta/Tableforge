package lobby_test

import (
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/testutil"
)

type fakeStore struct{ testutil.FakeStore }

func newFakeStore() *fakeStore {
	return &fakeStore{FakeStore: *testutil.NewFakeStore()}
}

func newTestService() (*lobby.Service, *fakeStore) {
	s := newFakeStore()
	reg := newFakeRegistry(&stubGame{id: "chess", minPlayers: 2, maxPlayers: 2})
	return lobby.New(s, reg), s
}
