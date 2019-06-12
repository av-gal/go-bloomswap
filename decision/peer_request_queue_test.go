package decision

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/ipfs/go-bitswap/wantlist"
	cid "github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	testutil "github.com/libp2p/go-libp2p-core/test"
)

func TestPushPop(t *testing.T) {
	prq := newPRQ()
	partner := testutil.RandPeerIDFatal(t)
	alphabet := strings.Split("abcdefghijklmnopqrstuvwxyz", "")
	vowels := strings.Split("aeiou", "")
	consonants := func() []string {
		var out []string
		for _, letter := range alphabet {
			skip := false
			for _, vowel := range vowels {
				if letter == vowel {
					skip = true
				}
			}
			if !skip {
				out = append(out, letter)
			}
		}
		return out
	}()
	sort.Strings(alphabet)
	sort.Strings(vowels)
	sort.Strings(consonants)

	// add a bunch of blocks. cancel some. drain the queue. the queue should only have the kept entries

	for _, index := range rand.Perm(len(alphabet)) { // add blocks for all letters
		letter := alphabet[index]
		t.Log(partner.String())

		c := cid.NewCidV0(u.Hash([]byte(letter)))
		prq.Push(partner, wantlist.Entry{Cid: c, Priority: math.MaxInt32 - index})
	}
	for _, consonant := range consonants {
		c := cid.NewCidV0(u.Hash([]byte(consonant)))
		prq.Remove(c, partner)
	}

	prq.fullThaw()

	var out []string
	for {
		received := prq.Pop()
		if received == nil {
			break
		}

		for _, entry := range received.Entries {
			out = append(out, entry.Cid.String())
		}
	}

	// Entries popped should already be in correct order
	for i, expected := range vowels {
		exp := cid.NewCidV0(u.Hash([]byte(expected))).String()
		if out[i] != exp {
			t.Fatal("received", out[i], "expected", expected)
		}
	}
}

// This test checks that peers wont starve out other peers
func TestPeerRepeats(t *testing.T) {
	prq := newPRQ()
	a := testutil.RandPeerIDFatal(t)
	b := testutil.RandPeerIDFatal(t)
	c := testutil.RandPeerIDFatal(t)
	d := testutil.RandPeerIDFatal(t)

	// Have each push some blocks

	for i := 0; i < 5; i++ {
		elcid := cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))
		prq.Push(a, wantlist.Entry{Cid: elcid})
		prq.Push(b, wantlist.Entry{Cid: elcid})
		prq.Push(c, wantlist.Entry{Cid: elcid})
		prq.Push(d, wantlist.Entry{Cid: elcid})
	}

	// now, pop off four entries, there should be one from each
	var targets []string
	var tasks []*peerRequestTask
	for i := 0; i < 4; i++ {
		t := prq.Pop()
		targets = append(targets, t.Target.Pretty())
		tasks = append(tasks, t)
	}

	expected := []string{a.Pretty(), b.Pretty(), c.Pretty(), d.Pretty()}
	sort.Strings(expected)
	sort.Strings(targets)

	t.Log(targets)
	t.Log(expected)
	for i, s := range targets {
		if expected[i] != s {
			t.Fatal("unexpected peer", s, expected[i])
		}
	}

	// Now, if one of the tasks gets finished, the next task off the queue should
	// be for the same peer
	for blockI := 0; blockI < 4; blockI++ {
		for i := 0; i < 4; i++ {
			// its okay to mark the same task done multiple times here (JUST FOR TESTING)
			tasks[i].Done(tasks[i].Entries)

			ntask := prq.Pop()
			if ntask.Target != tasks[i].Target {
				t.Fatal("Expected task from peer with lowest active count")
			}
		}
	}
}

func TestCleaningUpQueues(t *testing.T) {
	partner := testutil.RandPeerIDFatal(t)
	var entries []wantlist.Entry
	for i := 0; i < 5; i++ {
		entries = append(entries, wantlist.Entry{Cid: cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))})
	}

	prq := newPRQ()

	// push a block, pop a block, complete everything, should be removed
	prq.Push(partner, entries...)
	task := prq.Pop()
	task.Done(task.Entries)
	task = prq.Pop()

	if task != nil || len(prq.partners) > 0 || prq.pQueue.Len() > 0 {
		t.Fatal("Partner should have been removed because it's idle")
	}

	// push a block, remove each of its entries, should be removed
	prq.Push(partner, entries...)
	for _, entry := range entries {
		prq.Remove(entry.Cid, partner)
	}
	task = prq.Pop()

	if task != nil || len(prq.partners) > 0 || prq.pQueue.Len() > 0 {
		t.Fatal("Partner should have been removed because it's idle")
	}

}
