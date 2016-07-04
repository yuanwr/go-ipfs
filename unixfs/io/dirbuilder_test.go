package io

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"

	"golang.org/x/net/context"
)

func shuffle(arr []string) {
	for i := 0; i < len(arr); i++ {
		a := rand.Intn(len(arr))
		b := rand.Intn(len(arr))
		arr[a], arr[b] = arr[b], arr[a]
	}
}

func makeDir(ds dag.DAGService, size int) ([]string, *dirShard, error) {
	s := NewDirShard(ds, 256)

	var dirs []string
	for i := 0; i < size; i++ {
		dirs = append(dirs, fmt.Sprintf("DIRNAME%d", i))
	}

	rand.Seed(time.Now().UnixNano())
	shuffle(dirs)

	for i := 0; i < len(dirs); i++ {
		nd := &dag.Node{}
		ds.Add(nd)
		err := s.Insert(dirs[i], nd)
		if err != nil {
			return nil, nil, err
		}
	}

	return dirs, s, nil
}

func TestDirBuilding(t *testing.T) {
	ds := mdtest.Mock()
	s := NewDirShard(ds, 256)

	_, s, err := makeDir(ds, 200)
	if err != nil {
		t.Fatal(err)
	}

	nd, err := s.Node()
	if err != nil {
		t.Fatal(err)
	}

	//printDag(ds, nd, 0)

	k, err := nd.Key()
	if err != nil {
		t.Fatal(err)
	}

	if k.B58String() != "QmRgnPgLmvkbyFxQXr7BBm9VFdNRWGRteYFxfDMAebDJEG" {
		t.Fatal("output didnt match what we expected")
	}
}

func TestRemoveElems(t *testing.T) {
	ds := mdtest.Mock()
	dirs, s, err := makeDir(ds, 500)
	if err != nil {
		t.Fatal(err)
	}

	shuffle(dirs)

	for _, d := range dirs {
		err := s.Remove(d)
		if err != nil {
			t.Fatal(err)
		}
	}

	nd, err := s.Node()
	if err != nil {
		t.Fatal(err)
	}

}

func printDag(ds dag.DAGService, nd *dag.Node, depth int) {
	padding := strings.Repeat(" ", depth)
	fmt.Println("{")
	for _, l := range nd.Links {
		fmt.Printf("%s%s: %s", padding, l.Name, l.Hash.B58String())
		ch, err := ds.Get(context.Background(), key.Key(l.Hash))
		if err != nil {
			panic(err)
		}

		printDag(ds, ch, depth+1)
	}
	fmt.Println(padding + "}")
}
