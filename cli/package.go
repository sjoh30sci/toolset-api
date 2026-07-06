package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// packageCmd builds a portable tarball containing docker-compose, configs,
// docs, and the running CLI binary so the whole stack can be redistributed.
func packageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "package [output-path]",
		Short: "Package toolset for distribution",
		Long: `Create a portable tarball containing docker-compose, configs, and CLI binary.

Example:
  toolset package ./my-toolset.tar.gz
  toolset package  # defaults to ./toolset-<version>-<timestamp>.tar.gz`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath := fmt.Sprintf("toolset-%s-%d.tar.gz", Version, time.Now().Unix())
			if len(args) > 0 {
				outputPath = args[0]
			}

			// Resolve the project root (where docker-compose.yml lives) so the
			// command works regardless of the current working directory.
			root, err := findComposeDir()
			if err != nil {
				return fmt.Errorf("cannot locate project root: %w", err)
			}

			if err := createPackage(outputPath, root); err != nil {
				return fmt.Errorf("failed to create package: %w", err)
			}

			fmt.Printf("\u2713 Package created: %s\n", outputPath)
			fmt.Printf("  Extract with: tar -xzf %s\n", filepath.Base(outputPath))
			fmt.Printf("  Then run: cd toolset && docker-compose up -d\n")
			return nil
		},
	}
}

// createPackage writes a gzip-compressed tar archive of the distributable
// project files (rooted at root) plus the current CLI binary.
func createPackage(outputPath, root string) error {
	tarFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gzWriter := gzip.NewWriter(tarFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	baseDir := "toolset"
	filesToInclude := []string{
		"docker-compose.yml",
		"docker-compose.override.yml.example",
		".env.example",
		"config.yaml.example",
		"Makefile",
		"README.md",
		"LICENSE",
		"docs",
		"tools",
	}

	for _, fileOrDir := range filesToInclude {
		src := filepath.Join(root, fileOrDir)
		if _, statErr := os.Stat(src); os.IsNotExist(statErr) {
			// Skip optional artifacts (e.g. LICENSE) that may not be present.
			continue
		}
		if err := addToTar(tarWriter, root, fileOrDir, baseDir); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", fileOrDir, err)
		}
	}

	// Add the running CLI binary at toolset/bin/toolset so recipients can run
	// it directly without a separate download.
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	if err := addFileToTar(tarWriter, binaryPath, filepath.Join(baseDir, "bin", "toolset")); err != nil {
		return fmt.Errorf("failed to add CLI binary: %w", err)
	}

	return nil
}

// addToTar walks rel (relative to root) and writes every file/dir into the tar
// under baseDir, preserving the relative layout. Paths in the archive always
// use forward slashes per the tar spec.
func addToTar(tw *tar.Writer, root, rel, baseDir string) error {
	source := filepath.Join(root, rel)
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relToRoot, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		tarPath := filepath.ToSlash(filepath.Join(baseDir, relToRoot))

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = tarPath

		if info.IsDir() {
			header.Name += "/"
			return tw.WriteHeader(header)
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

// addFileToTar writes a single file into the archive at target, marking it
// executable (0755) since it is used for the CLI binary.
func addFileToTar(tw *tar.Writer, source, target string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    filepath.ToSlash(target),
		Size:    info.Size(),
		Mode:    0o755,
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, f)
	return err
}
