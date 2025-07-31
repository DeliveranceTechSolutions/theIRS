package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Xmler struct {
    Record map[string][]string
    Writer *csv.Writer
}

var wg sync.WaitGroup
var rw sync.RWMutex
var myInt atomic.Int64

const (
    MAXPROCS = 12
)

func ParseXMLs() {
    runtime.GOMAXPROCS(MAXPROCS)
    // _, header := Load()  // Load function not defined
    header := []string{"FileName", "EIN", "OrganizationName", "TaxYear", "ReturnType"} // Simple header
    sheet, err := os.Create("resolve.csv")
    if err != nil {
        panic(err)
    }
    writer := csv.NewWriter(sheet)

    xmler := &Xmler{
        Record: make(map[string][]string), // Initialize the map
        Writer: writer,
    }
    xmler.Writer.Write(header) 
    xmler.Writer.Flush()

    pathway := "./data/990_zips/"
    reader, err := os.ReadDir(pathway)
    if err != nil {
        fmt.Println(err)
    }

    re := regexp.MustCompile(`.zip`)
    for _, zipper := range reader {
        if !re.Match([]byte(zipper.Name())) {
            zReader, err := os.ReadDir(pathway + zipper.Name())
            if err != nil {
                fmt.Println(err)
            }
            wg.Add(1)
            go xmler.generateRows(pathway + zipper.Name(), zReader, &wg)
        }
    }

    wg.Wait()
}

func (x Xmler) generateRows(root string, files []os.DirEntry, wg *sync.WaitGroup) {
    defer wg.Done()


    for _, file := range files {
        if file.IsDir() {
            return
        }
        f, err := os.Open(root + "/" + file.Name())
        if err != nil {
            panic(err)
        }


        decoder := xml.NewDecoder(f)
        x.flatten(xml.StartElement{}, decoder, "")
    }

    return
}

func (x Xmler) flatten(element xml.StartElement, decoder *xml.Decoder, prefix string) {
    var lastTag string
    for {
        tok, err := decoder.Token()
        if err == io.EOF {
            rw.Lock()
            var row []string
            for _, data := range x.Record {
                length := len(data)
                if length > 1 {
                    var insert string
                    for _, b := range data {
                        insert += b
                    }

                    row = append(row, insert)   
                } else if length == 1 {
                    row = append(row, string(data[0]))
                } else if length == 0 {
                    row = append(row, "")
                }
            }

            x.Writer.Write(row)
            x.Writer.Flush()
            rw.Unlock()
            //x.Record, _ = Load() 
            myInt.Add(1)
            fmt.Println(myInt.Load())
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        switch t := tok.(type) {
        case xml.StartElement:
            fullTag := prefix + "." + t.Name.Local
            lastTag = t.Name.Local
            element = t
            x.flatten(t, decoder, fullTag)

        case xml.CharData:
            rw.Lock()
            val := strings.TrimSpace(string(t))
            if val != "" {
                x.Record[prefix] = append(x.Record[prefix], val)
            }
            if _, ok := x.Record[lastTag]; !ok {
                x.Record[lastTag] = []string{}
            }
            rw.Unlock()

        case xml.EndElement:
            if t.Name.Local == element.Name.Local {
                return
            }
        }
    }
}

func UnzipXMLs() {
    pathway := "./data/990_zips/"

    reader, err := os.ReadDir(pathway)
    if err != nil {
        fmt.Println(err)
    }

    for _, zipper := range reader {
        template := pathway + zipper.Name()
        unzipXMLs(template, template[:len(template) - 4])
    }
}

func unzipXMLs(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer func() {
        if err := r.Close(); err != nil {
            panic(err)
        }
    }()

    os.MkdirAll(dest, 0777)
    extractAndWriteFile := func(f *zip.File) error {
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer func() {
            if err := rc.Close(); err != nil {
                panic(err)
            }
        }()

        path := filepath.Join(dest, f.Name)
        if !strings.HasPrefix(path, filepath.Clean(dest) + string(os.PathSeparator)) {
            return fmt.Errorf("illegal file path: %s", path)
        }

        if f.FileInfo().IsDir() {
            os.MkdirAll(path, f.Mode())
        } else {
            os.MkdirAll(filepath.Dir(path), f.Mode())
            f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
            if err != nil {
                return err
            }
            defer func() {
                if err := f.Close(); err != nil {
                    panic(err)
                }
            }()

            _, err = io.Copy(f, rc)
            if err != nil {
                return err
            }
        }
        return nil
    }

    for _, f := range r.File {
        err := extractAndWriteFile(f)
        if err != nil {
            return err
        }
    }

    return nil
}

