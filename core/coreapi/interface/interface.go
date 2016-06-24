package iface

import (
	"errors"
	"io"

	core "github.com/ipfs/go-ipfs/core"

	mhash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type CoreAPI interface {
	Context() context.Context
	IpfsNode() *core.IpfsNode     // XXX temporary
	Cat(Path) (Data, error)       // http GET
	Ls(Path) ([]Link, error)      // http GET
	Add(DataForAdd) (Path, error) // http POST
}

type Path interface {
	String() string
	Segments() []string
}

type Link interface {
	Name() string
	Size() uint64
	Hash() mhash.Multihash
}

type DataForAdd interface {
	io.ReadCloser
}

type Data interface {
	DataForAdd
	io.Seeker
}

var ErrDir = errors.New("object is a directory")
var ErrOffline = errors.New("can't resolve, ipfs node is offline")
