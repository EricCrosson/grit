package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"

	"github.com/jmalloc/grit/src/grit"
	"github.com/jmalloc/grit/src/index"
	"github.com/urfave/cli"
)

func clone(c grit.Config, idx *index.Index, ctx *cli.Context) error {
	ep, err := getCloneEndpoint(c, ctx)
	if err != nil {
		return err
	}

	dir, err := getCloneDir(c, ctx, ep)
	if err != nil {
		return err
	}

	opts := &git.CloneOptions{URL: ep.Actual}
	r, err := git.PlainClone(dir, false /* isBare */, opts)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		_ = os.RemoveAll(dir)
		return err
	}

	fmt.Fprintln(ctx.App.Writer, dir)

	if r != nil {
		err := setupTracking(r, dir)
		if err != nil {
			return err
		}
	}

	return idx.Add(dir, index.All())
}

func setupTracking(r *git.Repository, dir string) error {
	head, err := r.Head()
	if err != nil {
		return err
	}

	if !head.IsBranch() {
		return nil
	}

	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "\n[branch \"%s\"]\n", head.Name().Short())
	fmt.Fprintf(buf, "\tremote = origin\n")
	fmt.Fprintf(buf, "\tmerge = %s\n", head.Name())

	p := path.Join(dir, ".git", "config")
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = buf.WriteTo(f)
	return err
}

func getCloneEndpoint(c grit.Config, ctx *cli.Context) (grit.Endpoint, error) {
	slugOrURL := ctx.Args().First()
	if slugOrURL == "" {
		return grit.Endpoint{}, notEnoughArguments
	}

	source := ctx.String("source")

	normalized, err := transport.NewEndpoint(slugOrURL)
	if err == nil {
		if source != "" {
			return grit.Endpoint{}, usageError("can not combine --source with a URL")
		}

		return grit.Endpoint{
			Actual:     slugOrURL,
			Normalized: normalized,
		}, nil
	}

	if source != "" {
		if t, ok := c.Clone.Sources[source]; ok {
			return t.Resolve(slugOrURL)
		}
		return grit.Endpoint{}, unknownSource(source)
	}

	if ep, ok := probeForURL(c, ctx, slugOrURL); ok {
		return ep, nil
	}

	return grit.Endpoint{}, silentFailure
}

func probeForURL(c grit.Config, ctx *cli.Context, slug string) (grit.Endpoint, bool) {
	var sources []string
	var endpoints []grit.Endpoint

	probeSources(c, slug, func(n string, ep grit.Endpoint) {
		sources = append(sources, n)
		endpoints = append(endpoints, ep)
	})

	if i, ok := choose(ctx.App.Writer, sources); ok {
		return endpoints[i], true
	}

	return grit.Endpoint{}, false
}

func getCloneDir(c grit.Config, ctx *cli.Context, ep grit.Endpoint) (string, error) {
	target := ctx.String("target")

	if ctx.Bool("golang") {
		if target == "" {
			return grit.EndpointToGoDir(ep)
		}

		return "", usageError("can not combine --target with --golang")
	}

	if target == "" {
		return grit.EndpointToDir(c.Clone.Root, ep)
	}

	return filepath.Abs(target)
}
