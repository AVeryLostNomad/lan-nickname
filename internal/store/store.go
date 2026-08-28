package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/joey/lan-nicknames/internal/config"
	"github.com/joey/lan-nicknames/internal/protocol"
)

const cacheFileName = "peers.json"

type Peer struct {
	ID         string               `json:"id"`
	Nickname   string               `json:"nickname"`
	Alias      string               `json:"alias"`
	SSHHostKey string               `json:"ssh_host_key,omitempty"`
	Addresses  map[string]time.Time `json:"addresses"`
}

type Snapshot struct {
	UpdatedAt time.Time `json:"updated_at"`
	Peers     []Peer    `json:"peers"`
}

type Store struct {
	mu    sync.RWMutex
	peers map[string]*Peer
}

func New() *Store {
	return &Store{peers: make(map[string]*Peer)}
}

func CachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFileName), nil
}

func Load() (*Store, error) {
	path, err := CachePath()
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read peer cache: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(contents, &snapshot); err != nil {
		return nil, fmt.Errorf("decode peer cache: %w", err)
	}
	result := New()
	for index := range snapshot.Peers {
		peer := snapshot.Peers[index]
		if peer.ID == "" || peer.Alias == "" || len(peer.Addresses) == 0 {
			continue
		}
		copy := peer
		result.peers[peer.ID] = &copy
	}
	return result, nil
}

func (store *Store) Observe(announcement protocol.Announcement, ip net.IP, now time.Time) {
	address := ip.String()
	store.mu.Lock()
	defer store.mu.Unlock()
	peer, found := store.peers[announcement.ID]
	if !found {
		peer = &Peer{ID: announcement.ID, Addresses: make(map[string]time.Time)}
		store.peers[announcement.ID] = peer
	}
	if peer.Nickname != announcement.Nickname || peer.Alias != announcement.Alias {
		peer.Nickname = announcement.Nickname
		peer.Alias = announcement.Alias
		peer.Addresses = make(map[string]time.Time)
	}
	peer.SSHHostKey = announcement.SSHHostKey
	peer.Addresses[address] = now
}

func (store *Store) Prune(before time.Time) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for id, peer := range store.peers {
		for address, lastSeen := range peer.Addresses {
			if lastSeen.Before(before) {
				delete(peer.Addresses, address)
			}
		}
		if len(peer.Addresses) == 0 {
			delete(store.peers, id)
		}
	}
}

func (store *Store) Snapshot() Snapshot {
	store.mu.RLock()
	defer store.mu.RUnlock()
	peers := make([]Peer, 0, len(store.peers))
	for _, peer := range store.peers {
		copy := Peer{
			ID:         peer.ID,
			Nickname:   peer.Nickname,
			Alias:      peer.Alias,
			SSHHostKey: peer.SSHHostKey,
			Addresses:  make(map[string]time.Time, len(peer.Addresses)),
		}
		for address, lastSeen := range peer.Addresses {
			copy.Addresses[address] = lastSeen
		}
		peers = append(peers, copy)
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Alias == peers[j].Alias {
			return peers[i].ID < peers[j].ID
		}
		return peers[i].Alias < peers[j].Alias
	})
	return Snapshot{UpdatedAt: time.Now(), Peers: peers}
}

func (store *Store) Save() error {
	path, err := CachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := config.OwnForInvoker(filepath.Dir(path)); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(store.Snapshot(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode peer cache: %w", err)
	}
	contents = append(contents, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, contents, 0o600); err != nil {
		return fmt.Errorf("write peer cache: %w", err)
	}
	if err := config.OwnForInvoker(temporary); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace peer cache: %w", err)
	}
	return nil
}

func ReadSnapshot(ttl time.Duration, now time.Time) (Snapshot, error) {
	store, err := Load()
	if err != nil {
		return Snapshot{}, err
	}
	store.Prune(now.Add(-ttl))
	return store.Snapshot(), nil
}
