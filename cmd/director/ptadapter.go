package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

type FileMap map[string][]byte

func getObsCertificates(configNum int) FileMap {
	const tarballPath = "templates/obfs-10k-certs.tar.gz"

	pattern := fmt.Sprintf("state.%d/*", configNum)

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

func getObsCertificatePart(b []byte) string {
	re := regexp.MustCompile(`cert=(.+) `)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	panic("could not find cert in file")
}

func getObfsPTAdapterServerTemplate() []byte {
	ptAdapterTemplate, err := os.ReadFile("templates/ptadapter-obs-server.template")
	if err != nil {
		log.Fatal(err)
	}
	return ptAdapterTemplate
}

func getObfsPTAdapterClientTemplate(certFile, bridgeHostname string) []byte {
	ptAdapterTemplate, err := os.ReadFile("templates/ptadapter-obs-client.template")
	if err != nil {
		log.Fatal(err)
	}

	tmpl, err := template.New("obsClientTemplate").Parse(string(ptAdapterTemplate))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			Server string
			Cert   string
		}{
			Server: bridgeHostname + ":8080",
			Cert:   certFile,
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()

}
