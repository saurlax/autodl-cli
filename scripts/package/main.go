// Package binaries for all supported platforms using the same build path in CI and releases.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	version := flag.String("version", "dev", "Version embedded in binaries and archive names")
	out := flag.String("out", "dist", "Output directory")
	flag.Parse()
	if err := build(*version, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(version, out string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "autodl-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var sums strings.Builder
	for _, target := range []struct{ os, arch string }{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"}} {
		name := "autodl"
		if target.os == "windows" {
			name += ".exe"
		}
		binary := filepath.Join(tmp, name)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", binary, "./cmd/autodl")
		// Replace inherited cross-build settings rather than creating duplicate env keys.
		for _, e := range os.Environ() {
			key := strings.ToUpper(strings.SplitN(e, "=", 2)[0])
			if key != "GOOS" && key != "GOARCH" && key != "CGO_ENABLED" {
				cmd.Env = append(cmd.Env, e)
			}
		}
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0", "GOOS="+target.os, "GOARCH="+target.arch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("build %s/%s: %w", target.os, target.arch, err)
		}
		archive := fmt.Sprintf("autodl_%s_%s_%s", version, target.os, target.arch)
		files := []struct {
			name, path string
			mode       int64
		}{{name, binary, 0755}, {"README.md", "README.md", 0644}, {"LICENSE", "LICENSE", 0644}}
		if target.os == "windows" {
			archive += ".zip"
		} else {
			archive += ".tar.gz"
		}
		path := filepath.Join(out, archive)
		if err = pack(path, files, target.os == "windows"); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", h.Sum(nil), archive)
		fmt.Println("Built", archive)
	}
	return os.WriteFile(filepath.Join(out, "checksums.txt"), []byte(sums.String()), 0644)
}

func pack(path string, files []struct {
	name, path string
	mode       int64
}, windows bool) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
	}()
	stamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if windows {
		zw := zip.NewWriter(f)
		for _, file := range files {
			b, e := os.ReadFile(file.path)
			if e != nil {
				zw.Close()
				return e
			}
			hdr := &zip.FileHeader{Name: file.name, Method: zip.Deflate, Modified: stamp}
			hdr.SetMode(os.FileMode(file.mode))
			w, e := zw.CreateHeader(hdr)
			if e != nil {
				zw.Close()
				return e
			}
			if _, e = w.Write(b); e != nil {
				zw.Close()
				return e
			}
		}
		return zw.Close()
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, file := range files {
		b, e := os.ReadFile(file.path)
		if e != nil {
			tw.Close()
			gz.Close()
			return e
		}
		if e = tw.WriteHeader(&tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(b)), ModTime: stamp}); e != nil {
			tw.Close()
			gz.Close()
			return e
		}
		if _, e = tw.Write(b); e != nil {
			tw.Close()
			gz.Close()
			return e
		}
	}
	if err = tw.Close(); err != nil {
		gz.Close()
		return err
	}
	return gz.Close()
}
