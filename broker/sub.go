/**********************************************************************************
* Copyright (c) 2009-2017 Misakai Ltd.
* This program is free software: you can redistribute it and/or modify it under the
* terms of the GNU Affero General Public License as published by the  Free Software
* Foundation, either version 3 of the License, or(at your option) any later version.
*
* This program is distributed  in the hope that it  will be useful, but WITHOUT ANY
* WARRANTY;  without even  the implied warranty of MERCHANTABILITY or FITNESS FOR A
* PARTICULAR PURPOSE.  See the GNU Affero General Public License  for  more details.
*
* You should have  received a copy  of the  GNU Affero General Public License along
* with this program. If not, see<http://www.gnu.org/licenses/>.
************************************************************************************/

package broker

import (
	"sync"

	"github.com/emitter-io/emitter/broker/cluster"
	"github.com/emitter-io/emitter/security"
)

// Ssid represents a subscription ID which contains a contract and a list of hashes
// for various parts of the channel.
type Ssid []uint32

// NewSsid creates a new Ssid.
func NewSsid(contract uint32, c *security.Channel) Ssid {
	ssid := make([]uint32, 0, len(c.Query)+1)
	ssid = append(ssid, uint32(contract))
	ssid = append(ssid, c.Query...)
	return ssid
}

// Contract gets the contract part from Ssid.
func (s Ssid) Contract() uint32 {
	return uint32(s[0])
}

// GetHashCode combines the ssid into a single hash.
func (s Ssid) GetHashCode() uint32 {
	h := s[0]
	for _, i := range s[1:] {
		h ^= i
	}
	return h
}

// ------------------------------------------------------------------------------------

// Subscriber is a value associated with a subscription.
type Subscriber interface {
	Send(ssid []uint32, channel []byte, payload []byte) error
}

// ------------------------------------------------------------------------------------

// Subscribers represents a subscriber set which can contain only unique values.
type Subscribers []Subscriber

// AddUnique adds a subscriber to the set.
func (s *Subscribers) AddUnique(value Subscriber) {
	if s.Contains(value) == false {
		*s = append(*s, value)
	}
}

// Contains checks whether a subscriber is in the set.
func (s *Subscribers) Contains(value Subscriber) bool {
	for _, v := range *s {
		if v == value {
			return true
		}
	}
	return false
}

// Subscription represents a topic subscription.
type Subscription struct {
	Ssid       Ssid       // Gets or sets the SSID (parsed channel) for this subscription.
	Channel    string     // Gets or sets the channel for this subscription.
	Subscriber Subscriber // Gets or sets the subscriber for this subscription.
}

// ------------------------------------------------------------------------------------

// SubscriptionCounters represents a subscription counting map.
type SubscriptionCounters struct {
	sync.Mutex
	m map[uint32]*subCounter
}

type subCounter struct {
	Ssid    Ssid
	Channel string
	Counter int
}

// NewSubscriptionCounters creates a new container.
func NewSubscriptionCounters() *SubscriptionCounters {
	return &SubscriptionCounters{
		m: make(map[uint32]*subCounter),
	}
}

// Increment increments the subscription counter.
func (s *SubscriptionCounters) Increment(ssid Ssid, channel string) {
	s.Lock()
	defer s.Unlock()

	m := s.getOrCreate(ssid, channel)
	m.Counter++
}

// Decrement decrements a subscription counter.
func (s *SubscriptionCounters) Decrement(ssid Ssid) {
	s.Lock()
	defer s.Unlock()

	key := ssid.GetHashCode()
	if m, exists := s.m[key]; exists {
		m.Counter--

		// Remove if there's no subscribers left
		if m.Counter <= 0 {
			delete(s.m, ssid.GetHashCode())
		}
	}
}

// All returns all subscriptions by copying the underlying map into a slice
func (s *SubscriptionCounters) All() []cluster.Subscription {
	s.Lock()
	defer s.Unlock()

	clone := make([]cluster.Subscription, 0, len(s.m))
	for _, m := range s.m {
		clone = append(clone, cluster.Subscription{
			Ssid:    m.Ssid,
			Channel: []byte(m.Channel),
		})
	}

	return clone
}

// getOrCreate retrieves a single subscription meter or creates a new one.
func (s *SubscriptionCounters) getOrCreate(ssid Ssid, channel string) (meter *subCounter) {
	key := ssid.GetHashCode()
	if m, exists := s.m[key]; exists {
		return m
	}

	meter = &subCounter{
		Ssid:    ssid,
		Channel: channel,
		Counter: 0,
	}
	s.m[key] = meter
	return
}
