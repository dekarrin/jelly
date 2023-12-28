// Package owdb provides OrbweaverDB stores. OrbweaverDB is a NoSQL database
// that holds web analytics data, originally for the araneastats project. It is
// a very simple combined OLAP and OLTP system that is safe for concurrent
// access.
//
// This is something like the DAO for this project. It has nothing to do with
// web crawling; quite the opposite, this spider sits in its web and waits for
// requests to come to *it*. Fundamentally this is architected as a multi-valued
// time-series oriented DB system.
//
// Use [Open] to create a [Store] that persists to a file on disk. The Store
// provides a full-featured database. The data within can be saved to disk by
// calling [Store.Persist] at appropriate times, and when a Store is no longer
// in use, [Store.Close] is called to end all current operations. An in-memory
// Store is obtained either by creating a &Store{} manually or calling [Import]
// to create one from previously-obtained bytes.
package owdb

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dekarrin/rezi/v2"
)

// Requester holds information on an HTTP request client.
type Requester struct {

	// Address is the IP address of the client.
	Address net.IP

	// Country is the name of the country that the source IP address is from, as
	// per geolocation lookup of the Address.
	Country string

	// City is the name of the city that the source IP address is from, as per
	// geolocation lookup of the Address.
	City string
}

func (r Requester) MarshalBinary() ([]byte, error) {
	var enc []byte

	enc = append(enc, rezi.MustEnc(r.Address)...)
	enc = append(enc, rezi.MustEnc(r.Country)...)
	enc = append(enc, rezi.MustEnc(r.City)...)

	return enc, nil
}

func (r *Requester) UnmarshalBinary(data []byte) error {
	rr, err := rezi.NewReader(bytes.NewBuffer(data), nil)
	if err != nil {
		return err
	}

	var decoded Requester

	// address
	err = rr.Dec(&decoded.Address)
	if err != nil {
		return rezi.Wrapf(0, "address: %s", err)
	}

	// country
	err = rr.Dec(&decoded.Country)
	if err != nil {
		return rezi.Wrapf(0, "country: %s", err)
	}

	// city
	err = rr.Dec(&decoded.City)
	if err != nil {
		return rezi.Wrapf(0, "city: %s", err)
	}

	*r = decoded

	return nil
}

// Equal returns whether other is a Requester with the same properties as r.
func (r Requester) Equal(other interface{}) bool {
	var otherReq Requester

	if v, ok := other.(Requester); ok {
		otherReq = v
	} else if vPtr, ok := other.(*Requester); ok {
		if vPtr == nil {
			return false
		}
		otherReq = *vPtr
	} else {
		return false
	}

	// avoid using net.IP.Equal because it will return true for an IPv6 and IPv4
	// form address that points to the same place, but for us, that's an actual
	// change.
	if r.Address == nil {
		if otherReq.Address != nil {
			return false
		}
	} else if otherReq.Address == nil {
		return false
	} else if r.Address.String() != otherReq.Address.String() {
		return false
	}

	if r.City != otherReq.City {
		return false
	}
	if r.Country != otherReq.Country {
		return false
	}

	return true
}

func (r Requester) String() string {
	if r.Address == nil {
		return "<?.?.?.?>"
	}
	return fmt.Sprintf("<%s>", r.Address)
}

// Hit is a single hit on a website from a particular IP address, which may or
// may not be unique.
type Hit struct {
	// Time is the time that the event was recorded.
	Time time.Time

	// Host is an identifier of the host that the client was accessing. It is
	// usually a DNS name but may also be an IP address.
	Host string

	// Resource is the identifier of the resource within the host that the
	// client was attempting to access. This is usually a path for HTTP servers
	// or something similar.
	Resource string

	// Client is information on the HTTP client who made the request.
	Client Requester
}

func (h Hit) MarshalBinary() ([]byte, error) {
	var enc []byte

	enc = append(enc, rezi.MustEnc(h.Time)...)
	enc = append(enc, rezi.MustEnc(h.Host)...)
	enc = append(enc, rezi.MustEnc(h.Resource)...)
	enc = append(enc, rezi.MustEnc(h.Client)...)

	return enc, nil
}

func (h *Hit) UnmarshalBinary(data []byte) error {
	rr, err := rezi.NewReader(bytes.NewBuffer(data), nil)
	if err != nil {
		return err
	}

	var decoded Hit

	// decode time
	err = rr.Dec(&decoded.Time)
	if err != nil {
		return rezi.Wrapf(0, "time: %s", err)
	}

	// decode host
	err = rr.Dec(&decoded.Host)
	if err != nil {
		return rezi.Wrapf(0, "host: %s", err)
	}

	// decode resource
	err = rr.Dec(&decoded.Resource)
	if err != nil {
		return rezi.Wrapf(0, "resource: %s", err)
	}

	// decode client
	err = rr.Dec(&decoded.Client)
	if err != nil {
		return rezi.Wrapf(0, "client: %s", err)
	}

	decoded.normalizeForDB()

	*h = decoded
	return nil
}

// normalize modifies the hit to be ready for entry into the DB. It sets the
// time to be suitable as a key. This should always be called after any update
// or insertion where fields are modified.
func (hit *Hit) normalizeForDB() {
	hit.Time = hit.Time.UTC().Round(0)
}

// Equal returns whether other is a Hit with the same proprties as hit.
func (hit Hit) Equal(other interface{}) bool {
	var oHit Hit

	if v, ok := other.(Hit); ok {
		oHit = v
	} else if vPtr, ok := other.(*Hit); ok {
		if vPtr == nil {
			return false
		}
		oHit = *vPtr
	} else {
		return false
	}

	if hit.Time != oHit.Time {
		return false
	}
	if hit.Host != oHit.Host {
		return false
	}
	if hit.Resource != oHit.Resource {
		return false
	}
	if !hit.Client.Equal(oHit.Client) {
		return false
	}

	return true
}

func (hit Hit) String() string {
	return fmt.Sprintf("[%s %s %s %s]", hit.Time.Format(time.RFC3339), hit.Host, hit.Resource, hit.Client)
}

// Store holds analytics data and provides access to both storage (OLTP) and
// analytics of events. The zero-value is in-memory only, but one that syncs to
// disk on calls to [Store.Persist] can be made by calling [Open] or setting
// [Store.DataDir] manually.
//
// Store is safe to use from multiple goroutines concurrently. It serializes
// access to internal storage.
//
// The zero-value is a Store with no Hits in it ready for immediate use as an
// in-memory database whose Persist function does not save it to disk. Store
// must not be copied once created.
type Store struct {
	// DataFile is the file on disk that the store will store state data in when
	// [Store.Persist] is called. It will be set automatically when the Store is
	// created with a call to [Open].
	//
	// If set to the empty string, calls to [Store.Persist] will have no effect.
	// This allows for in-memory database behavior.
	DataFile string

	mtx    sync.RWMutex
	closed bool

	// hits is the source of truth of the Store. Indexes will refer to Hits by
	// indexes into this slice.
	//
	// elements are ordered by time of the Hit.
	hits []Hit
}

// Open creates a new Store that will persist itself to the given data file. If
// the file already exists, its entire contents are loaded into a new *Store
// which is then returned. If the file does not exist, it will be created.
//
// The returned Store will have its DataFile member set to the given file. This
// does not make it so the returned Store will automatically save its contents
// to disk, rather [Store.Persist] or [Store.Close] must be called manually to
// flush it.
//
// If file is set to the empty string, the Store will be opened in in-memory
// mode and calls to Persist and Close will only finalize any pending changes
// and will not write to disk.
func Open(file string) (*Store, error) {
	s := &Store{}
	if file == "" {
		return s, nil
	}

	dbData, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if err == nil {
		s, err = Import(dbData)
		if err != nil {
			return nil, fmt.Errorf("load data: %w", err)
		}
	} else if os.IsNotExist(err) {
		// quick check to see if later writing would fail due to permissions.
		f, err := os.Create(file)
		if err != nil {
			return nil, fmt.Errorf("create new: %w", err)
		}
		defer f.Close()
		_, err = f.Write([]byte{0x00})
		if err != nil {
			return nil, fmt.Errorf("initial write: %w", err)
		}
	}

	return s, nil
}

// ImportFile reads the bytes in the given file and returns the result of
// calling [Import] on the read bytes.
func ImportFile(file string) (*Store, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return Import(data)
}

// MarshalBinary converts the store to a binary bytes representation of itself.
// These bytes may be saved to disk or loaded into another Store with
// UnmarshalBinary.
//
// This function is not concurrent safe and requires a read lock. Users of Store
// should prefer calling [Store.Persist] (or [Store.Export] if the exact bytes
// are needed) instead, which safely obtain one and handle any other required
// operations.
func (s *Store) MarshalBinary() ([]byte, error) {
	if s == nil {
		return []byte{}, nil
	}

	var enc []byte

	enc = append(enc, rezi.MustEnc(s.hits)...)

	return enc, nil
}

// UnmarshalBinary converts a binary byte representation of a Store located at
// the start of data and uses it to set the values on the Store.
//
// This function is not concurrent safe and requires a write lock. Users of
// Store should prefer calling [Open] or [Import] to create a Store from bytes,
// which safely handle obtaining synchronization primitives and any other
// required operations.
func (s *Store) UnmarshalBinary(data []byte) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal to nil Store")
	}

	rr, err := rezi.NewReader(bytes.NewBuffer(data), nil)
	if err != nil {
		return err
	}

	err = rr.Dec(&s.hits)
	if err != nil {
		return rezi.Wrapf(0, "hits: %s", err)
	}

	return nil
}

// Export exports all data to bytes that can be later decoded with [Open] or
// [Store.Import].
func (s *Store) Export() ([]byte, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.exportUnsafe()
}

func (s *Store) exportUnsafe() ([]byte, error) {
	if s.closed {
		return nil, fmt.Errorf("operation called on closed *Store")
	}

	return rezi.Enc(s)
}

// Persist waits for any pending data updates in the Store to be applied and
// then saves the data, generally to disk. Persistance to disk will occur if
// Store.DataFile is set to a non-empty string. If Store.DataFile is the empty
// string (i.e. if Store is in in-memory mode), calling Persist will only do
// whatever is necessary to make any pending changes visible to future requests.
//
// Persist is not automatically called; the user must do so themselves at the
// correct frequency. It is recommended it be called after each logical "batch"
// of operations.
//
// When Persist is called, all data in s is marshaled to bytes and saved to
// disk, regardless of whether any changes occurred to the data since it was
// last persisted or loaded. This has performance implications, especially as
// the amount of data grows large.
func (s *Store) Persist() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.persistUnsafe()
}

// persistUnsafe does actual work of Persist. It assumes the caller has aqcuired
// a write lock on the data mutex.
func (s *Store) persistUnsafe() error {
	if s.closed {
		return fmt.Errorf("operation called on closed *Store")
	}

	// we don't currently have a concept of "pending operations" besides just
	// the write lock to be sure we are the most recent writer and any pending
	// readers wait for us, and the caller should have already aquired a write
	// lock so should be fine.
	//
	// however, when/if we do get pending operations, waiting for them to close
	// would go here.

	if s.DataFile == "" {
		// nowhere to persist to. done.
		return nil
	}

	// first, copy the old file so we have a backup in case somefin goes wrong
	buFile, err := createFileBackup(s.DataFile)
	if err != nil {
		if os.IsNotExist(err) {
			// that's fine actually, but set buFile to empty so we know we don't
			// have one to delete later
			buFile = ""
		} else {
			return fmt.Errorf("create backup: %w", err)
		}
	}

	// open the data file
	wf, err := os.Create(s.DataFile)
	if err != nil {
		return fmt.Errorf("create data file: %w", err)
	}
	defer wf.Close()
	w := bufio.NewWriter(wf)

	// now that we have an open data file, get the data and write it all to it.
	// TODO: could probably do this in parallel with backup creation and data
	// file open.
	dataBytes, err := s.exportUnsafe()
	if err != nil {
		return fmt.Errorf("get data bytes: %w", err)
	}

	_, err = w.Write(dataBytes)
	if err != nil {
		return fmt.Errorf("write data file: %w", err)
	}

	// at end of everyfin, if successful, remove the backup.
	if buFile != "" {
		os.Remove(buFile)
	}

	return nil
}

// Close ends the Store connection. It automatically persists any unflushed
// changes (if persistence is configured via the DataFile member) and releases
// any other outstanding resources.
//
// After Close returns, the Store cannot be used again, regardless of whether
// the returned error is nil.
//
// If the Store has already been closed, calling this method will have no effect
// and the returned error will be nil.
func (s *Store) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.closed {
		return nil
	}

	err := s.persistUnsafe()

	// close the connection even if err is not nil; we don't want the Store to
	// be usable after return.
	s.closed = true

	if err != nil {
		return fmt.Errorf("persist data to disk: %w", err)
	}
	return nil
}

// Import loads the given data bytes into a new in-memory Store. The data bytes
// must have been created by a prior call to [Store.Export].
//
// The returned Store will be in-memory only by default, and will not persist to
// disk when [Store.Persist] is called. To change this, set DataFile on the
// returned Store.
func Import(data []byte) (*Store, error) {
	s := &Store{}

	_, err := rezi.Dec(data, s)
	return s, err
}

func (s *Store) String() string {
	if s == nil {
		return "Store<nil>"
	}
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	recCount := len(s.hits)
	recS := "s"
	if recCount == 1 {
		recS = ""
	}

	var sb strings.Builder

	sb.WriteString("Store<")
	if s.closed {
		sb.WriteString("(CLOSED), ")
	}
	sb.WriteString(fmt.Sprintf("%d row%s", recCount, recS))
	if s.DataFile == "" {
		sb.WriteString(", in-memory")
	} else {
		sb.WriteString(fmt.Sprintf(", %q", s.DataFile))
	}
	sb.WriteRune('>')
	return sb.String()
}

// DataString returns a string containing all current data in the store. It can
// be useful for debugging. If the Store has already been closed, the data will
// not be shown.
func (s *Store) DataString() string {
	if s == nil {
		return "Store<nil>"
	}
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if s.closed {
		return "Store<(closed)>"
	}

	var sb strings.Builder

	sb.WriteString("Store<")
	if len(s.hits) > 0 {
		sb.WriteRune('\n')
	}
	for i := range s.hits {
		sb.WriteString("* ")
		sb.WriteString(s.hits[i].String())
		sb.WriteRune('\n')
	}
	sb.WriteRune('>')
	return sb.String()
}

// Select selects all hits that match the given Filter. If there are no matches,
// a slice with length 0 will be returned along with a nil error.
func (s *Store) Select(f Filter) ([]Hit, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("operation called on closed *Store")
	}

	ids := s.applyFilter(f)
	if len(ids) == 0 {
		return nil, nil
	}

	var selected []Hit
	for _, id := range ids {
		selected = append(selected, s.hits[id])
	}

	return selected, nil
}

// Update applies a transformation function to hits to get a new one. All hits
// that match the given filter will be passed to the given function in order to
// create a new one. Update returns the number of records that match the filter
// as well as the number of records actually changed by the provided function.
//
// While it is possible to, within the function, apply one's own checks and
// return the Hit unchanged when it doesn't meet it, this will result in poor
// performance than if the Filter is used to limit the query to only those Hits
// which are to be modified.
//
// If the update function modifies an indexed field, a performance hit is
// incurred as the indexes will then need to be modified.
func (s *Store) Update(f Filter, update func(Hit) Hit) (matched, updated int, err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.closed {
		return 0, 0, fmt.Errorf("operation called on closed *Store")
	}

	// find ones to update
	matchedIDs := s.applyFilter(f)
	if len(matchedIDs) == 0 {
		return 0, 0, nil
	}

	var timeIndexChanges map[int]struct{} = make(map[int]struct{})
	var updateCount int

	// apply updates
	for _, id := range matchedIDs {
		oldHit := s.hits[id]
		newHit := update(s.hits[id])
		newHit.normalizeForDB()

		// did an update actually occur?
		if !newHit.Equal(oldHit) {
			// if time changed, we need to update the index.
			if newHit.Time != oldHit.Time {
				timeIndexChanges[id] = struct{}{}
			}

			// set old equal to the new one
			s.hits[id] = newHit
			updateCount++
		}
	}

	// fix any indexes that just broke, if needed
	for len(timeIndexChanges) > 0 {
		// pop off the first ID
		var id int
		for k := range timeIndexChanges {
			id = k
			break
		}
		delete(timeIndexChanges, id)

		updatedHit := s.hits[id]
		newPoint := s.findInsertionPoint(updatedHit, id)

		// if findInsertionPoint is same as old, there's nothing to do
		if newPoint == id {
			continue
		}

		// need to change any indexes that are being updated by this shift
		// operation
		updatedTimeChanges := map[int]struct{}{}

		// if new point is AFTER current point, subtract one
		if newPoint > id {
			newPoint--

			// move all before/at the new point but after old point left-by-one.
			for i := id; i < newPoint; i++ {
				s.hits[i] = s.hits[i+1]

				// if it's an ID that needs to be re-indexed, shift it
				if _, ok := timeIndexChanges[i+1]; ok {
					// decrement ID to match what just happened.
					updatedTimeChanges[i] = struct{}{}
					delete(timeIndexChanges, i+1)
				}
			}
		} else {
			// nmove all after/at the point but before old point right-by-one
			for i := id; i > newPoint; i-- {
				s.hits[i] = s.hits[i-1]

				// if it's an ID that needs to be re-indexed, shift it
				if _, ok := timeIndexChanges[i-1]; ok {
					// increment ID to match what just happened.
					updatedTimeChanges[i] = struct{}{}
					delete(timeIndexChanges, i-1)
				}
			}
		}

		// now place updated at new point
		s.hits[newPoint] = updatedHit

		// if we just moved an internal ID of somefin we need to later
		// re-index, come back to it.
		if len(updatedTimeChanges) > 0 {
			for k := range timeIndexChanges {
				updatedTimeChanges[k] = struct{}{}
			}
			timeIndexChanges = updatedTimeChanges
		}
	}

	// indexes are now updated, return count of actually updated
	return len(matchedIDs), updateCount, nil
}

// Delete removes a hit from the store. All hits that match the given Filter
// will be deleted. If f is nil, all hits will be considered to match. Returns
// the number of data points deleted. If no hit of that time exists, nothing is
// performed and no error is returned.
//
// This operation runs in O(n) with respect to the number of elements in the DB.
func (s *Store) Delete(f Filter) (int, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.closed {
		return 0, fmt.Errorf("operation called on closed *Store")
	}

	// find it
	toDel := s.applyFilter(f)
	if len(toDel) == 0 {
		return 0, nil
	}

	shift := 1
	for i := toDel[shift-1]; i < len(s.hits)-shift; i++ {
		// if the one at i+shift is also to be deleted, we increment shift until
		// this is no longer the case.
		for len(toDel) > shift && i+shift == toDel[shift] {
			shift++
		}
		if i+shift >= len(s.hits) {
			// done
			break
		}

		s.hits[i] = s.hits[i+shift]
	}

	// now remove everyfin at the end, glub!
	// all but the last len(toDel).
	updated := make([]Hit, len(s.hits)-len(toDel))
	copy(updated, s.hits)
	s.hits = updated

	return len(toDel), nil
}

// Insert adds a new hit to the store. The time of the hit is not modified and
// is used to determine the storage location of the data.
//
// This operation runs in O(n) with respect to the number of elements in the
// DB.
func (s *Store) Insert(h Hit) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.closed {
		return fmt.Errorf("operation called on closed *Store")
	}

	h.normalizeForDB()

	// where to put it?
	insertAt := s.findInsertionPoint(h)

	// TODO: THIS is probs the true source of inefficiency. Oh my gog, look at
	// this f8cking gar8age! If this were a linked list, it would be so much
	// faster to insert!
	//
	// but, glub, way slower to search!
	//
	// Perhaps A Solutions Which Combines The Two? We Could Place A Table Of
	// Pointers At The Head Of The First Item Or The Structure Which Contains
	// The First Node If We Are Comfortable Doing Such A Thing.
	//
	// maybe

	// first, make room by inserting an empty hit so we don't do a full copy
	// unless capacity was overrun. In addition, makes it so we don't gotta
	// manually copy.
	s.hits = append(s.hits, Hit{})

	// now, shift everyfin right up to the insertion idx
	for i := len(s.hits) - 1; i > insertAt; i-- {
		s.hits[i] = s.hits[i-1]
	}

	// finally, insert our value
	s.hits[insertAt] = h

	return nil
}

// findInsertionPoint returns the index of the given hit where it should be
// placed as a new Hit.
func (s *Store) findInsertionPoint(h Hit, skipIDs ...int) int {
	insertAt := 0

	// TODO: if date-based partitions ever get made, they would be checked
	// instead of every date. But right now we need a *functional* DB, not a
	// performant one.
	//
	// We could at least swap to sort.Search ::::/ That would make this O(log n)
	// instead of O(n)!
	for i := len(s.hits) - 1; i >= 0; i-- {
		if sliceIndexOf(skipIDs, i) > -1 {
			continue
		}
		if s.hits[i].Time.Before(h.Time) {
			insertAt = i + 1
			break
		}
	}
	return insertAt
}

// applyFilter applies the given filter to the entire store and returns a slice
// of internal IDs of items that it matches. If a nil Filter is provided, it
// will match all IDs.
//
// This method is NOT protected by mutex; ensure calling code is.
func (s *Store) applyFilter(f Filter) []int {
	if f == nil {
		f = Where{}
	}

	// query planning
	//
	// rule 1: always use time index, and get possible bounds from filter
	timeBounds := f.TimeIndexLimits()
	if timeBounds.IsImpossible(time.Time.After) {
		// index bounds impossible, just search the whole thing unbounded
		timeBounds = Limits[time.Time]{}
	}

	// end query planning, now do scan on PK index (Time)
	start := -1
	end := -1

	for i, h := range s.hits {
		if timeBounds.Contains(h.Time, time.Time.Equal, time.Time.After) {
			if start == -1 {
				start = i
			}
		} else if start != -1 {
			// its sorted, we have found the end of our range
			end = i
			break
		}
	}
	if start == -1 {
		// no matches
		return nil
	}
	if end == -1 {
		// select EVERYFIN from start
		end = len(s.hits)
	}

	var selected []int
	for i := start; i < end; i++ {
		// check which match the filter and keep those
		if f.Matches(s.hits[i]) {
			selected = append(selected, i)
		}
	}

	return selected
}

func sliceIndexOf[E comparable](sl []E, item E) int {
	for i := range sl {
		if sl[i] == item {
			return i
		}
	}

	return -1
}
