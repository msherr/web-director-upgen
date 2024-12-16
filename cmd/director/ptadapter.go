package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"text/template"
)

type FileMap map[string][]byte

//go:embed templates/obfs-10k-certs.tar.gz
var tarballBytes []byte

//go:embed templates/ptadapter-obs-client.template
var ptAdapterObsClientTemplateBytes []byte

//go:embed templates/ptadapter-obs-server.template
var ptAdapterObsServerTemplateBytes []byte

//go:embed templates/ptadapter-proteus-client.template
var ptAdapterProteusClientTemplateBytes []byte

//go:embed templates/ptadapter-proteus-server.template
var ptAdapterProteusServerTemplateBytes []byte

const startingPortNum = 8080

func getObsCertificates(configNum int) FileMap {

	pattern := fmt.Sprintf("state.%d/*", configNum)

	// Create a gzip reader
	br := bytes.NewReader(tarballBytes)
	gzipReader, err := gzip.NewReader(br)
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

func getObsCertificatePart(b []byte) []byte {
	re := regexp.MustCompile(`cert=(.+) `)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Bytes()
		matches := re.FindSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	panic("could not find cert in file")
}

func getObfsPTAdapterServerTemplate(iterationNum int) []byte {
	tmpl, err := template.New("obsServerTemplate").Parse(string(ptAdapterObsServerTemplateBytes))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			ListenPort int
		}{
			ListenPort: iterationNum + startingPortNum,
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()

}

func getObfsPTAdapterClientTemplate(certBytes []byte, bridgeHostname string, iterationNum int) []byte {
	tmpl, err := template.New("obsClientTemplate").Parse(string(ptAdapterObsClientTemplateBytes))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			Server string
			Cert   string
		}{
			Server: fmt.Sprintf("%s:%d", bridgeHostname, iterationNum+startingPortNum),
			Cert:   string(certBytes),
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()
}

func getProteusPTAdapterServerTemplate(optionString string, iterationNum int) []byte {

	tmpl, err := template.New("proteusServerTemplate").Parse(string(ptAdapterProteusServerTemplateBytes))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			Options    string
			ListenPort int
		}{
			Options:    optionString,
			ListenPort: iterationNum + startingPortNum,
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()
}

func getProteusPTAdapterClientTemplate(optionString, bridgeHostname string, iterationNum int) []byte {
	tmpl, err := template.New("proteusClientTemplate").Parse(string(ptAdapterProteusClientTemplateBytes))
	if err != nil {
		log.Fatal(err)
	}

	var parsedTemplate bytes.Buffer
	err = tmpl.Execute(&parsedTemplate,
		struct {
			Server  string
			Options string
		}{
			Server:  fmt.Sprintf("%s:%d", bridgeHostname, iterationNum+startingPortNum),
			Options: optionString,
		})
	if err != nil {
		log.Fatal(err)
	}

	return parsedTemplate.Bytes()
}
