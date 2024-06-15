package main

import (
	"log"
	"os"
)

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
		return nil, err
	}

	tmpl, err := template.New("obsServerTemplate").Parse(string(obsServerTemplate))
	if err != nil {
		log.Fatal(err)
		return nil, err
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
		return nil, err
	}

	return parsedTemplate.Bytes(), nil
}
*/
