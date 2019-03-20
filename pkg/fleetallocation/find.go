// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fleetallocation

import (
	"agones.dev/agones/pkg/apis/stable/v1alpha1"
)

// nodeCount is just a convenience data structure for
// keeping relevant GameServer counts about Nodes
type nodeCount struct {
	ready     int64
	allocated int64
}

// findReadyGameServerForAllocation is a O(n) implementation to find a GameServer with priority
// defined in the comparator function.
// nolint: dupl
func findReadyGameServerForAllocation(gsList []*v1alpha1.GameServer, comparator func(bestCount, currentCount *nodeCount) bool, gameServerName string) *v1alpha1.GameServer {
	counts := map[string]*nodeCount{}
	// track potential gameservers, one for each node
	allocatableGameServers := map[string]*v1alpha1.GameServer{}

	// try to allocate the specific gameServer if gameServerName is provided
	// otherwise count up the number of allocated and ready game servers that exist
	// also, since we're already looping through, track one Ready GameServer
	// per node, so we can use that as a short list to allocate from
	for _, gs := range gsList {
		if gs.DeletionTimestamp.IsZero() &&
			gameServerName != "" &&
			(gs.Status.State == v1alpha1.GameServerStateAllocated || gs.Status.State == v1alpha1.GameServerStateReady) {
			// Allocate the requested gameServer
			if gs.Name == gameServerName {
				return gs
			}
		} else if gs.DeletionTimestamp.IsZero() &&
			(gs.Status.State == v1alpha1.GameServerStateAllocated || gs.Status.State == v1alpha1.GameServerStateReady) {
			_, ok := counts[gs.Status.NodeName]
			if !ok {
				counts[gs.Status.NodeName] = &nodeCount{}
			}

			if gs.Status.State == v1alpha1.GameServerStateAllocated {
				counts[gs.Status.NodeName].allocated++
			} else if gs.Status.State == v1alpha1.GameServerStateReady {
				counts[gs.Status.NodeName].ready++
				allocatableGameServers[gs.Status.NodeName] = gs
			}
		}
	}

	// Could not find the requested gameServer to allocate
	if gameServerName != "" {
		return nil
	}

	// track the best node count
	var bestCount *nodeCount
	// the current GameServer from the node with the most GameServers (allocated, ready)
	var bestGS *v1alpha1.GameServer

	for nodeName, count := range counts {
		// count.ready > 0: no reason to check if we don't have ready GameServers on this node
		// bestGS == nil: if there is no best GameServer, then this node & GameServer is the always the best
		if count.ready > 0 && (bestGS == nil || comparator(bestCount, count)) {
			bestCount = count
			bestGS = allocatableGameServers[nodeName]
		}
	}

	return bestGS
}

// packedComparator prioritises Nodes with GameServers that are allocated, and then Nodes with the most
// Ready GameServers -- this will bin pack allocated game servers together.
func packedComparator(bestCount, currentCount *nodeCount) bool {
	if currentCount.allocated == bestCount.allocated && currentCount.ready > bestCount.ready {
		return true
	} else if currentCount.allocated > bestCount.allocated {
		return true
	}

	return false
}

// distributedComparator is the inverse of the packed comparator,
// looking to distribute allocated gameservers on as many nodes as possible.
func distributedComparator(bestCount, currentCount *nodeCount) bool {
	return !packedComparator(bestCount, currentCount)
}
