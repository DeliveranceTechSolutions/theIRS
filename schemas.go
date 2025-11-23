package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocomply/xsd2go/pkg/xsd2go"
)

func UnzipSchemas() error {
    entries, err := os.ReadDir("./data/990_xsd")
    if err != nil {
        return fmt.Errorf("read dir: %w", err)
    }

    dstRoot := "./data/990_xsd/output"
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        zipPath := filepath.Join("./data/990_xsd", entry.Name())
        archive, err := zip.OpenReader(zipPath)
        if err != nil {
            return fmt.Errorf("open zip %q: %w", zipPath, err)
        }

        // process and then close immediately
        if err := func() error {
            defer archive.Close()

            for _, f := range archive.File {
                destPath := filepath.Join(dstRoot, f.Name)
                // guard against ZipSlip
                if !strings.HasPrefix(destPath, filepath.Clean(dstRoot)+string(os.PathSeparator)) {
                    return fmt.Errorf("illegal file path: %s", destPath)
                }

                if f.FileInfo().IsDir() {
                    if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
                        return err
                    }
                    continue
                }

                if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
                    return err
                }

                outFile, err := os.OpenFile(
                    destPath,
                    os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
                    f.Mode(),
                )
                if err != nil {
                    return err
                }
                defer outFile.Close()

                rc, err := f.Open()
                if err != nil {
                    return err
                }
                defer rc.Close()

                if _, err := io.Copy(outFile, rc); err != nil {
                    return err
                }
            }
            return nil
        }(); err != nil {
            return err
        }
    }
    return nil
}

func GlobWalk(rootDir, pattern string) ([]string, error) {
    var matches []string

    err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
        if walkErr != nil {
            return walkErr
        }
        if d.IsDir() {
            return nil
        }
        matched, err := filepath.Match(pattern, filepath.Base(path))
        if err != nil {
            return err
        }
        if !matched {
            return nil
        }

        // === Begin per‐file conversion logic ===
        // Remember current working dir
        cwd, err := os.Getwd()
        if err != nil {
            return err
        }
        // Switch into the XSD’s directory so includes resolve
        schemaDir := filepath.Dir(path)
        if err := os.Chdir(schemaDir); err != nil {
            return fmt.Errorf("chdir to %q: %w", schemaDir, err)
        }
        // Restore cwd when done
        defer os.Chdir(cwd)

        // Call xsd2go.Convert on the base filename
        if err := xsd2go.Convert(
            filepath.Base(path),                     // just the file name now
            "main",                                   // or whatever package name you want
            filepath.Join(cwd, "data/990_xsd/output/generated_templates"),
            nil,
        ); err != nil {
            log.Printf("xsd2go failed for %q: %v", path, err)
            // Continue processing other files
        }
        // === End per‐file conversion logic ===

        matches = append(matches, path)
        return nil
    })
    if err != nil {
        return nil, err
    }
    return matches, nil
}


//func GlobWalk(rootDir, pattern string) ([]string, error) {
//    var matches []string
//
//    // 1) collect all matching files
//    err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
//        if walkErr != nil {
//            return walkErr
//        }
//        if d.IsDir() {
//            return nil
//        }
//        ok, err := filepath.Match(pattern, filepath.Base(path))
//        if err != nil {
//            return err
//        }
//        if ok {
//            matches = append(matches, path)
//        }
//        return nil
//    })
//    if err != nil {
//        return nil, err
//    }
//
//    // 2) call Convert once per file
//    for _, uri := range matches {
//        if err := xsd2go.Convert(
//            uri,
//            "main",                                              // package name
//            "./data/990_xsd/output/generated_templates",         // output dir
//            nil,                                                 // no overrides
//        ); err != nil {
//            return nil, fmt.Errorf("xsd2go conversion failed for %q: %w", uri, err)
//        }
//    }
//
//    return matches, nil
//}



func SchemaGenerator(uri string) error {
    err := xsd2go.Convert(
        uri,
        "github.com/synergos-systems/xsd2go",
        "./parser/xsd2go",
        nil,
    )
    if err != nil {
        return fmt.Errorf("xsd2go conversion failed: %w", err)
    }
    return nil
}
