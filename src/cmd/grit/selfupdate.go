package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/cavaliercoder/grab"
	humanize "github.com/dustin/go-humanize"
	"github.com/google/go-github/github"
	"github.com/jmalloc/grit/src/grit/update"
	"github.com/urfave/cli"
)

func selfUpdate(c *cli.Context) error {
	// setup a deadline first ...
	timeout := time.Duration(c.Int("timeout")) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	write(c, "searching for the latest release")

	gh := github.NewClient(nil)
	preRelease := c.Bool("pre-release")
	rel, err := update.FindLatest(ctx, gh, preRelease)
	if err != nil {
		if err == update.ErrReleaseNotFound && !preRelease {
			return errors.New(err.Error() + ", try --pre-release")
		}
		return err
	}

	latest, err := semver.NewVersion(rel.GetTagName())
	if err != nil {
		return err
	}

	cmp := latest.Compare(VERSION)

	if !c.Bool("force") {
		if cmp == 0 {
			write(c, "current version (%s) is up to date", VERSION)
			return nil
		} else if cmp < 0 {
			return fmt.Errorf(
				"latest published release (%s) is older than the current version (%s)",
				latest,
				VERSION,
			)
		}
	}

	actualBin, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("downloading version %s", latest)
	message := prefix
	messageLen := len(message)
	fmt.Fprint(c.App.Writer, message)

	archive, err := update.Download(
		ctx,
		grab.DefaultClient,
		rel,
		func(recv, total uint64) {
			r := float64(recv)
			t := float64(total)
			message = fmt.Sprintf(
				"%s (%d%%, %s / %s)",
				prefix,
				int(r/t*100.0),
				humanize.Bytes(recv),
				humanize.Bytes(total),
			)

			fmt.Fprint(c.App.Writer, "\r"+message)

			l := len(message)
			if messageLen > l {
				clr := strings.Repeat(" ", messageLen-l)
				fmt.Fprint(c.App.Writer, clr)
			}
			messageLen = l
		},
	)
	write(c, "")
	if err != nil {
		return err
	}

	latestBin := actualBin + "." + latest.String()
	backupBin := actualBin + "." + VERSION.String() + ".backup"

	err = update.Unpack(archive, latestBin)
	if err != nil {
		return err
	}

	err = os.Rename(actualBin, backupBin)
	if err != nil {
		return err
	}

	err = os.Rename(latestBin, actualBin)
	if err != nil {
		return os.Rename(backupBin, actualBin)
	}

	if cmp > 0 {
		write(c, "upgraded from version %s to %s", VERSION, latest)
	} else if cmp < 0 {
		write(c, "downgraded from version %s to %s", VERSION, latest)
	} else if VERSION.String() == latest.String() {
		write(c, "reinstalled version %s", VERSION)
	} else {
		write(c, "reinstalled version %s as %s", VERSION, latest)
	}

	return os.Remove(backupBin)
}