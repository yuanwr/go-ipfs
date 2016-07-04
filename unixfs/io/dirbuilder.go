package io

import (
	"bytes"
	"fmt"
	"math"

	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	proto "github.com/gogo/protobuf/proto"
	key "github.com/ipfs/go-ipfs/blocks/key"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	upb "github.com/ipfs/go-ipfs/unixfs/pb"
	"github.com/spaolacci/murmur3"
	"github.com/willf/bitset"
)

const (
	HashMurmur3 uint64 = iota
)

const ShardSplitTreshold = 1000
const DefaultShardWidth = 256

type Directory struct {
	dserv   mdag.DAGService
	dirnode *mdag.Node

	shard *dirShard
}

// NewEmptyDirectory returns an empty merkledag Node with a folder Data chunk
func NewEmptyDirectory() *mdag.Node {
	nd := new(mdag.Node)
	nd.SetData(format.FolderPBData())
	return nd
}

// NewDirectory returns a Directory. It needs a DAGService to add the Children
func NewDirectory(dserv mdag.DAGService) *Directory {
	db := new(Directory)
	db.dserv = dserv
	db.dirnode = NewEmptyDirectory()
	return db
}

func NewDirectoryFromNode(dserv mdag.DAGService, nd *mdag.Node) (*Directory, error) {
	pbd, err := format.FromBytes(nd.Data())
	if err != nil {
		return nil, err
	}

	switch pbd.GetType() {
	case format.TDirectory:
		return &Directory{
			dserv:   dserv,
			dirnode: nd,
		}, nil
	case format.THAMTShard:
		shard, err := DirShardFromNode(dserv, nd)
		if err != nil {
			return nil, err
		}

		return &Directory{
			dserv: dserv,
			shard: shard,
		}, nil
	default:
		return nil, fmt.Errorf("merkledag node was not a directory or shard")
	}
}

// AddChild adds a (name, key)-pair to the root node.
func (d *Directory) AddChild(ctx context.Context, name string, nd *mdag.Node) error {
	if d.shard == nil {
		if len(d.dirnode.Links) < ShardSplitTreshold {
			return d.dirnode.AddNodeLinkClean(name, nd)
		} else {
			d.shard = NewDirShard(d.dserv, DefaultShardWidth)
			for _, lnk := range d.dirnode.Links {
				cnd, err := d.dserv.Get(ctx, key.Key(lnk.Hash))
				if err != nil {
					return err
				}

				err = d.shard.Insert(lnk.Name, cnd)
				if err != nil {
					return err
				}
			}

			d.dirnode = nil
		}
	}

	return d.shard.Insert(name, nd)
}

func (d *Directory) Links() ([]*mdag.Link, error) {
	if d.shard == nil {
		return d.dirnode.Links, nil
	} else {
		return d.shard.EnumLinks()
	}
}

func (d *Directory) Find(ctx context.Context, name string) (*mdag.Node, error) {
	if d.shard == nil {
		lnk, err := d.dirnode.GetNodeLink(name)
		if err != nil {
			return nil, err
		}

		return d.dserv.Get(ctx, key.Key(lnk.Hash))
	}

	return d.shard.Find(name)
}

func (d *Directory) RemoveChild(ctx context.Context, name string) error {
	if d.shard == nil {
		return d.dirnode.RemoveNodeLink(name)
	} else {
		return d.shard.Remove(name)
	}
}

// GetNode returns the root of this Directory
func (d *Directory) GetNode() (*mdag.Node, error) {
	if d.shard == nil {
		return d.dirnode, nil
	}

	return d.shard.Node()
}

type dirShard struct {
	nd *mdag.Node

	bitfield *bitset.BitSet

	children []child

	tableSize    uint
	tableSizeLg2 int

	collapsed    bool
	prefixPadStr string

	dserv mdag.DAGService
}

type child interface {
	Node() (*mdag.Node, error)
	Label() string
}

func NewDirShard(dserv mdag.DAGService, size uint) *dirShard {
	ds := mkDirShard(dserv, size)
	ds.bitfield = bitset.New(size)
	ds.children = make([]child, size)
	return ds
}

func mkDirShard(ds mdag.DAGService, size uint) *dirShard {
	maxpadding := fmt.Sprintf("%X", size-1)
	return &dirShard{
		tableSizeLg2: int(math.Log2(float64(size))),
		prefixPadStr: fmt.Sprintf("%%0%dX", len(maxpadding)),
		tableSize:    size,
		dserv:        ds,
	}
}

func DirShardFromNode(dserv mdag.DAGService, nd *mdag.Node) (*dirShard, error) {
	pbd, err := format.FromBytes(nd.Data())
	if err != nil {
		return nil, err
	}

	if pbd.GetType() != upb.Data_HAMTShard {
		return nil, fmt.Errorf("node was not a dir shard")
	}

	ds := mkDirShard(dserv, uint(pbd.GetFanout()))
	ds.nd = nd
	ds.collapsed = true
	ds.bitfield = bitset.New(0)
	_, err = ds.bitfield.ReadFrom(bytes.NewReader(pbd.GetData()))
	if err != nil {
		return nil, err
	}

	return ds, nil
}

func (ds *dirShard) Node() (*mdag.Node, error) {
	out := new(mdag.Node)

	for i, child := range ds.children {
		if child == nil {
			continue
		}

		cnd, err := child.Node()
		if err != nil {
			return nil, err
		}

		err = out.AddNodeLinkClean(ds.linkNamePrefix(i)+child.Label(), cnd)
		if err != nil {
			return nil, err
		}
	}
	buf := new(bytes.Buffer)
	ds.bitfield.WriteTo(buf)

	typ := upb.Data_HAMTShard
	data, err := proto.Marshal(&upb.Data{
		Type:     &typ,
		Fanout:   proto.Uint64(uint64(ds.tableSize)),
		HashType: proto.Uint64(HashMurmur3),
		Data:     buf.Bytes(),
	})
	if err != nil {
		return nil, err
	}

	out.SetData(data)

	_, err = ds.dserv.Add(out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

type shardValue struct {
	key string
	val *mdag.Node
}

func (sv *shardValue) Node() (*mdag.Node, error) {
	return sv.val, nil
}

func (sv *shardValue) Label() string {
	return sv.key
}

func hash(val []byte) []byte {
	h := murmur3.New64()
	h.Write(val)
	return h.Sum(nil)
}

func (ds *dirShard) Label() string {
	return ""
}

func (ds *dirShard) Insert(name string, nd *mdag.Node) error {
	hv := &hashBits{b: hash([]byte(name))}
	return ds.modifyHash(hv, name, nd)
}

func (ds *dirShard) Remove(name string) error {
	hv := &hashBits{b: hash([]byte(name))}
	return ds.modifyHash(hv, name, nil)
}

func (ds *dirShard) Find(name string) (*mdag.Node, error) {
	hv := &hashBits{b: hash([]byte(name))}

	return ds.findHash(hv, name)
}

func (ds *dirShard) getChild(i int) (child, error) {
	if ds.collapsed {
		panic("not yet implemented")
	} else {
		return ds.children[i], nil
	}
}

func (ds *dirShard) setChild(i int, c child) error {
	if ds.collapsed {
		panic("we dont do this yet")
	} else {
		ds.children[i] = c
		return nil
	}
}

func (ds *dirShard) findHash(hv *hashBits, key string) (*mdag.Node, error) {
	var out *mdag.Node
	err := ds.traverseHash(hv, key, func(sv *shardValue) error {
		out = sv.val
		return nil
	})

	return out, err
}

func (ds *dirShard) walkDown(hv *hashBits, key string, cb func(*shardValue) error) error {
	idx := hv.Next(ds.tableSizeLg2)
	if ds.bitfield.Test(idx) {
		cindex := ds.indexForBitPos(idx)

		child, err := ds.getChild(cindex)
		if err != nil {
			return nil, err
		}

		switch child := child.(type) {
		case *dirShard:
			return child.traverseHash(hv, key)
		case *shardValue:
			return cb(child)
		}
	}

	return nil, nil
}

func (ds *dirShard) EnumLinks() ([]*mdag.Link, error) {
	var links []*mdag.Link
	err := ds.walkTrie(func(sv *shardValue) error {
		k, err := sv.val.Key()
		if err != nil {
			return err
		}

		lnk := &mdag.Link{
			Name: sv.key,
		}
	})
}

func (ds *dirShard) walkTrie(cb func(*shardValue) error) error {
	return nil
}

func (ds *dirShard) modifyHash(hv *hashBits, key string, val *mdag.Node) error {
	idx := hv.Next(ds.tableSizeLg2)

	if ds.bitfield.Test(idx) {
		cindex := ds.indexForBitPos(idx)

		child, err := ds.getChild(cindex)
		if err != nil {
			return err
		}

		switch child := child.(type) {
		case *dirShard:
			err := child.modifyHash(hv, key, val)
			if err != nil {
				return err
			}

			if child.bitfield.Count() == 0 {
				ds.bitfield.Clear(idx)
				return ds.setChild(cindex, nil)
			}

			return nil
		case *shardValue:
			if val == nil {
				ds.bitfield.Clear(idx)
				ds.setChild(cindex, nil)
				return nil
			}

			ns := NewDirShard(ds.dserv, ds.tableSize)
			chhv := &hashBits{
				b:        hash([]byte(child.key)),
				consumed: hv.consumed,
			}

			err := ns.modifyHash(hv, key, val)
			if err != nil {
				return err
			}

			err = ns.modifyHash(chhv, child.key, child.val)
			if err != nil {
				return err
			}

			return ds.setChild(cindex, ns)
		default:
			panic("this shouldnt happen")
		}
	} else {
		if val == nil {
			return fmt.Errorf("entry not found")
		}

		ds.bitfield.Set(idx)
		cindex := ds.indexForBitPos(idx)

		sv := &shardValue{
			key: key,
			val: val,
		}

		return ds.setChild(cindex, sv)
	}
}

func (ds *dirShard) indexForBitPos(bp uint) int {
	if ds.collapsed {
		panic("not yet implemented")
	} else {
		return int(bp)
	}
}

func (ds *dirShard) linkNamePrefix(i int) string {
	return fmt.Sprintf(ds.prefixPadStr, i)
}

func (ds *dirShard) setBits() []uint {
	var out []uint
	for i := uint(0); i < ds.tableSize; i++ {
		if ds.bitfield.Test(i) {
			out = append(out, i)
		}
	}
	return out
}
