package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
)

type FileMap map[string][]byte

func getObsCertificates(expNumber int) FileMap {
	const tarballPath = "templates/obfs-10k-certs.tar.gz"

	pattern := fmt.Sprintf("state.%d/*", expNumber)

	file, err := os.Open(tarballPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Create a gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Fatal(err)
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	fileMap := make(FileMap)

	// Iterate over each file in the tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		// Check if the file matches the given pattern
		match, err := filepath.Match(pattern, header.Name)
		if err != nil {
			log.Fatal(err)
		}

		if match && header.Typeflag == tar.TypeReg {
			filename := filepath.Base(header.Name)
			fileMap[filename], err = io.ReadAll(tarReader)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	return fileMap
}

func getObsClientTemplate() []byte {
	obsClientTemplate, err := os.ReadFile("templates/ptadapter-obs-client.template")
	if err != nil {
		log.Fatal(err)
	}
	return obsClientTemplate
}

/*
func getObsServerTemplate(statedir string) ([]byte, error) {
	obsServerTemplate, err := os.ReadFile("templates/ptadapter-obs-server.template")
	if err != nil {
		log.Fatal(err)
	}

	tmpl, err := template.New("obsServerTemplate").Parse(string(obsServerTemplate))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			StateDir string
		}{
			StateDir: statedir,
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes(), nil
}
*/

func getObfsPTAdapterServerTemplate(configNum int) []byte {
	ptAdapterTemplate, err := os.ReadFile("templates/ptadapter-obs-server.template")
	if err != nil {
		log.Fatal(err)
	}

	tmpl, err := template.New("obsServerTemplate").Parse(string(ptAdapterTemplate))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			StateDir string
		}{
			StateDir: fmt.Sprintf("../generate-obfs-certs/state.%d", configNum),
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()
}
