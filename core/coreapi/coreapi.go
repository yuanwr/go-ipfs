package coreapi

import (
	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type Link struct {
	name string
	size uint64
	hash mh.Multihash
}

func (l *Link) Name() string       { return l.name }
func (l *Link) Size() uint64       { return l.size }
func (l *Link) Hash() mh.Multihash { return l.hash }

type CoreAPI struct {
	ctx  context.Context
	node *core.IpfsNode
}

func NewCoreAPI(ctx context.Context, node *core.IpfsNode) (coreiface.CoreAPI, error) {
	api := &CoreAPI{ctx: ctx, node: node}
	return api, nil
}

func (api *CoreAPI) Context() context.Context {
	return api.ctx
}

func (api *CoreAPI) IpfsNode() *core.IpfsNode {
	return api.node
}

func (api *CoreAPI) resolve(p coreiface.Path) (*dag.Node, error) {
	dagnode, err := core.Resolve(api.ctx, api.node, p.(path.Path))
	if err == core.ErrNoNamesys && !api.node.OnlineMode() {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}
	return dagnode, nil
}

func (api *CoreAPI) Cat(p coreiface.Path) (coreiface.Data, error) {
	if p.String() == "/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" {
		return nil, coreiface.ErrDir
	}
	dagnode, err := api.resolve(p)
	if err != nil {
		return nil, err
	}
	r, err := uio.NewDagReader(api.ctx, dagnode, api.node.DAG)
	if err == uio.ErrIsDir {
		return nil, coreiface.ErrDir
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

func (api *CoreAPI) Ls(p coreiface.Path) ([]coreiface.Link, error) {
	if p.String() == "/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" {
		return make([]coreiface.Link, 0), nil
	}
	dagnode, err := api.resolve(p)
	if err != nil {
		return nil, err
	}
	links := make([]coreiface.Link, len(dagnode.Links))
	for i, l := range dagnode.Links {
		links[i] = &Link{l.Name, l.Size, l.Hash}
	}
	return links, nil
}

func (api *CoreAPI) Add(data coreiface.DataForAdd) (coreiface.Path, error) {
	splitter := chunk.DefaultSplitter(data)
	dagnode, err := importer.BuildDagFromReader(api.node.DAG, splitter)
	if err != nil {
		return path.Path(""), err
	}
	k, err := api.node.DAG.Add(dagnode)
	if err != nil {
		return path.Path(""), err
	}
	return path.Path("/ipfs/" + k.String()), nil
}
