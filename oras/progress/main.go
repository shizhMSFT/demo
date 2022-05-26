package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"goloading/pkg/progress"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
)

var copyCommand = &cli.Command{
	Name:      "copy",
	ArgsUsage: "<source> <folder>",
	Action:    runCopy,
}

func runCopy(ctx *cli.Context) error {
	src, err := remote.NewRepository(ctx.Args().Get(0))
	if err != nil {
		return err
	}
	dst, err := oci.New(ctx.Args().Get(1))
	if err != nil {
		return err
	}

	t := &tracker{
		Target:  dst,
		manager: progress.NewProgressManager(),
	}
	desc, err := oras.Copy(ctx.Context, src, src.Reference.ReferenceOrDefault(), t, "")
	if err != nil {
		return err
	}
	t.manager.Wait()
	fmt.Println("Copied", desc.Digest)
	return nil
}

type trackedReader struct {
	io.Reader
	offset     int64
	descriptor ocispec.Descriptor
	progress   progress.Progress
}

func (r *trackedReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.offset += int64(n)
	d := r.descriptor.Digest.Encoded()[:12]
	r.progress <- fmt.Sprintln(d, r.offset, "/", r.descriptor.Size)
	return
}

type tracker struct {
	oras.Target
	manager progress.ManagerProgress
}

func (t *tracker) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	p := t.manager.Add()
	defer close(p)
	r := &trackedReader{
		Reader:     content,
		descriptor: expected,
		progress:   p,
	}
	err := t.Target.Push(ctx, expected, r)
	d := expected.Digest.Encoded()[:12]
	if err != nil {
		p <- fmt.Sprintln(d, "Error:", err)
	} else {
		p <- fmt.Sprintln(d, "Downloaded")
	}
	return err
}

func main() {
	app := &cli.App{
		Name: "oras",
		Commands: []*cli.Command{
			copyCommand,
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
