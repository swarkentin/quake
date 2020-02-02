// Copyright 2020 Navibyte (https://navibyte.com). All rights reserved.
// Use of this source code is governed by a MIT-style license, see the LICENSE.

package usgs

import (
	"errors"
	"sync"
	"time"

	pb "github.com/navibyte/quake/api/v1"
)

// stat contains statistics about an cache entry
type stat struct {
	fetchCount int
	hitCount   int
}

// entry for caching fetched&parsed responses
type entry struct {
	mu                 sync.Mutex
	col                *pb.EarthquakeCollection
	expires            time.Time
	errCountSinceReset int
	lastErrTime        time.Time
	lastErr            error

	stat
}

const (
	maxTriesForRequest = 3
	maxErrorsTotal     = 10
	waitBeforeReset    = time.Hour
)

var (
	// cache entries identified by key generated by resolveCacheKey()
	// (access to each entry is synchronized by a mutex for a key)
	entries map[string]*entry

	// copies of entry stat (synchronized by one RW-mutex)
	statMutex  sync.RWMutex
	statCopies map[string]stat
)

// ErrCacheFailure is returned on cache failures
var ErrCacheFailure = errors.New("failure on caching earthquake collection")

// ErrNotFound is returned when identified earthquake was not found
var ErrNotFound = errors.New("earthquake not found")

// init cache entries for all key combinations
func init() {
	// init entries
	entries = make(map[string]*entry)
	for _, magn := range pb.Magnitude_value {
		for _, past := range pb.Past_value {
			key := resolveCacheKey(pb.Magnitude(magn), pb.Past(past))
			entries[key] = &entry{}
		}
	}
	// init stat
	statCopies = make(map[string]stat)
}

// cacheGetById returns a single earthquake (cached or fetched if no cache hit)
func cacheGetById(id string) (*pb.Earthquake, error) {

	// loop "hour", "day", "7days" and "30days" cached lists to find identified one
	var lastErr error
	for _, past := range pb.Past_value {
		if pb.Past(past) != pb.Past_PAST_UNSPECIFIED {
			// get full collection for given "past" value
			col, err := cacheGetList(
				pb.Magnitude_MAGNITUDE_ALL, pb.Past(past))
			if err != nil {
				lastErr = err
			} else {
				// got collection, loop to find matching with id
				for _, eq := range col.Features {
					if eq.Id == id {
						return eq, nil
					}
				}
			}
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNotFound
}

// cacheGetList returns cached data from entry (or fetched data if no cache hit)
func cacheGetList(magnitude pb.Magnitude, past pb.Past) (
	*pb.EarthquakeCollection, error) {

	// resolve cache key and entry
	key := resolveCacheKey(magnitude, past)
	entry := entries[key]
	if entry == nil {
		return nil, ErrCacheFailure
	}

	// synchronize access to an entry identified by the key
	// (note that it's on purpose to acquire lock for all the time
	// needed to access cache entry and to fecth/parse data if needed)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// return cached data if available and not yet expired
	if entry.col != nil {
		if time.Now().After(entry.expires) {
			entry.col = nil
		} else {
			// cache hit
			entry.hitCount++
			cacheSetStat(magnitude, past, entry.stat)
			return entry.col, nil
		}
	}

	// if maximum number of errors occurred some time ago, reset error counters
	if entry.errCountSinceReset >= maxErrorsTotal &&
		time.Now().After(entry.lastErrTime.Add(waitBeforeReset)) {

		entry.errCountSinceReset = 0
		entry.lastErr = nil
	}

	// could not get valid cache entry, so need to fetch data
	// (trying to fetch&parse for few times before giving up)
	round := 0
	for round < maxTriesForRequest && entry.errCountSinceReset < maxErrorsTotal {
		data, err := fetch(magnitude, past)
		if err != nil { // fetch error
			entry.errCountSinceReset++
			entry.lastErr = err
			entry.lastErrTime = time.Now()
		} else {
			// fetched data successfully, now trying to parse it
			col, err := ToEarthquakeCollection(data, true)
			if err != nil { // parse error
				entry.errCountSinceReset++
				entry.lastErr = err
				entry.lastErrTime = time.Now()
			} else {
				// got valid response, store to the cache entry and return it
				entry.col = col
				entry.fetchCount++
				cacheSetStat(magnitude, past, entry.stat)
				entry.expires = time.Now().Add(resolveMaxAge(magnitude, past))
				entry.errCountSinceReset = 0
				entry.lastErr = nil
				return col, nil
			}
		}
		round++
	}

	// did not succeed on getting valid response, return last error
	if entry.lastErr == nil {
		return nil, ErrCacheFailure
	}
	return nil, entry.lastErr
}

// cacheGetStat returns latest statistics about an entry
func cacheGetStat(magnitude pb.Magnitude, past pb.Past) stat {
	// when reading acquire a read lock for statistics
	statMutex.RLock()
	defer statMutex.RUnlock()
	st, ok := statCopies[resolveCacheKey(magnitude, past)]
	if !ok {
		return stat{}
	}
	return st
}

// cacheSetStat sets latest statistics fon an entry
func cacheSetStat(magnitude pb.Magnitude, past pb.Past, st stat) {
	// when writing acquire a regular lock for statistics
	statMutex.Lock()
	defer statMutex.Unlock()
	statCopies[resolveCacheKey(magnitude, past)] = st
}

func resolveCacheKey(magnitude pb.Magnitude, past pb.Past) string {
	return magnitude.String() + "@" + past.String()
}

func resolveMaxAge(magnitude pb.Magnitude, past pb.Past) time.Duration {
	switch past {
	case pb.Past_PAST_HOUR:
		return 3 * time.Minute
	case pb.Past_PAST_DAY:
		return 5 * time.Minute
	case pb.Past_PAST_7DAYS:
		return 10 * time.Minute
	case pb.Past_PAST_30DAYS:
		return 15 * time.Minute
	default:
		return 15 * time.Minute
	}
}
