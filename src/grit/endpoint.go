package grit

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
)

const slugSeparator = "/"

// EndpointTemplate is template for a Git repository URL.
type EndpointTemplate string

// Endpoint represents a Git clone endpoint, resolved from an EndpointTemplate.
type Endpoint struct {
	// The actual URL used to clone the repository.
	// This string will match the URL template from the configuration as closely
	// as possible.
	Actual string

	// The substituted and normalized endpoint template. SCP-style Git URLs
	// are converted to ssh:// URLs.
	Normalized transport.Endpoint
}

// Validate returns an error if the template is invalid.
func (t EndpointTemplate) Validate() error {
	_, err := t.virtualEndpoint()
	return err
}

// IsMatch returns true if may have been derived from the endpoint template.
func (t EndpointTemplate) IsMatch(e transport.Endpoint) bool {
	ep, err := t.virtualEndpoint()
	if err != nil {
		return false
	}

	// TODO: match slug heuristically
	return e.Scheme == ep.Scheme && e.Host == ep.Host
}

// virtualEndpoint returns a Git endpoint from the template as though we had
// a slug to resolve.
func (t EndpointTemplate) virtualEndpoint() (transport.Endpoint, error) {
	ep, err := t.Resolve("__virtual__")
	return ep.Normalized, err
}

// Resolve returns a URL from the template.
func (t EndpointTemplate) Resolve(slug string) (ep Endpoint, err error) {
	ep.Actual, err = t.replace(slug)

	if err == nil {
		ep.Normalized, err = transport.NewEndpoint(ep.Actual)
	}

	return
}

func (t EndpointTemplate) replace(slug string) (u string, err error) {
	funcs := map[string]interface{}{
		"slug": func() string { return slug },
		"env":  os.Getenv,
	}

	tmpl, err := template.
		New("url").
		Funcs(funcs).
		Parse(string(t))

	if err == nil {
		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, nil)
		u = buf.String()
	}

	return
}

// EndpointExists returns true if url is a Git repository.
func EndpointExists(ep Endpoint) (ok bool, err error) {
	cli, err := client.NewClient(ep.Normalized)
	if err != nil {
		return
	}

	sess, err := cli.NewUploadPackSession(ep.Normalized, nil)
	if err != nil {
		return
	}
	defer sess.Close()

	_, err = sess.AdvertisedReferences()

	switch err {
	case transport.ErrRepositoryNotFound:
		err = nil
	case transport.ErrEmptyRemoteRepository:
		err = nil
		ok = true
	case nil:
		ok = true
	}

	return
}

// EndpointToDir returns the absolute path for a clone of a repository.
func EndpointToDir(base string, ep transport.Endpoint) string {
	slug := EndpointToSlug(ep)
	parts := strings.Split(slug, slugSeparator)
	return path.Join(base, ep.Host, path.Join(parts...))
}

// EndpointToSlug returns the "slug" from ep.
func EndpointToSlug(ep transport.Endpoint) string {
	return strings.TrimSuffix(
		strings.TrimPrefix(ep.Path, slugSeparator),
		path.Ext(ep.Path),
	)
}

// ReplaceSlug returns a copy of ep with the slug changed to s.
func ReplaceSlug(ep transport.Endpoint, s string) transport.Endpoint {
	ext := path.Ext(ep.Path)
	ep.Path = slugSeparator + s + ext
	return ep
}

// MergeSlug returns a copy of ep with the slug changed to s. If s has less
// path atoms then the existing slug it is merged with the existing slug such
// that the original number of path atoms are retained.
func MergeSlug(ep transport.Endpoint, s string) transport.Endpoint {
	a := strings.Split(s, slugSeparator)

	slug := EndpointToSlug(ep)
	atoms := strings.Split(slug, slugSeparator)

	diff := len(atoms) - len(a)

	if diff > 0 {
		s = strings.Join(atoms[0:diff], slugSeparator) + slugSeparator + s
	}

	return ReplaceSlug(ep, s)
}

// EndpointIsSCP returns true if s is an SSH URL given in "SCP" style, that is
// git@github.com:jmalloc/grit.git, as opposed to ssh://git@github.com/jmalloc/grit.git.
func EndpointIsSCP(s string) bool {
	ep, err := transport.NewEndpoint(s)

	return err == nil &&
		ep.Scheme == "ssh" &&
		!strings.HasPrefix(s, "ssh://")
}

// EndpointToSCP converts a normalized ssh:// endpoint URL to an SCP-style URL.
func EndpointToSCP(ep transport.Endpoint) (string, error) {
	if ep.Scheme != "ssh" {
		return "", errors.New("not an SSH endpoint")
	}

	return fmt.Sprintf(
		"%s@%s:%s",
		ep.User,
		ep.Host,
		strings.TrimPrefix(ep.Path, slugSeparator),
	), nil
}
