// Create the README.md file based on README.md.gotmpl and the yaml data in the flavour and feature directories
package main

import (
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"text/template"
)

type TemplateData struct {
	Features map[string]YamlDescriptor
	Flavours map[string]YamlDescriptor
}

func fetchDocData(basePath string, filenamePattern string) (map[string]YamlDescriptor, error) {
	docDatas := make(map[string]YamlDescriptor)

	if subDirs, err := filepath.Glob(basePath); err != nil {
		return docDatas, err
	} else {
		for _, dir := range subDirs {
			if _, err := os.Stat(filepath.Join(dir, filenamePattern)); err == nil {
				if yamlFile, err := os.ReadFile(filepath.Join(dir, filenamePattern)); err != nil {
					return docDatas, err
				} else {
					var descriptor YamlDescriptor
					if err := yaml.Unmarshal(yamlFile, &descriptor); err != nil {
						return docDatas, err
					}

					docDatas[filepath.Base(dir)] = descriptor
				}

			}
		}
	}

	return docDatas, nil
}

func main() {
	//var rootPath string
	//if ex, err := os.Executable(); err == nil {
	//	rootPath = filepath.Join(filepath.Dir(ex), "..")
	//} else {
	//	panic(err)
	//}

	templateData := TemplateData{}

	if docData, err := fetchDocData(filepath.Join("feature", "*"), "feature.yaml"); err != nil {
		panic(fmt.Sprintf("Can not fetch feature docdata: %s", err))
	} else {
		templateData.Features = docData
	}

	if docData, err := fetchDocData(filepath.Join("flavour", "*"), "flavour.yaml"); err != nil {
		panic(fmt.Sprintf("Can not fetch feature docdata: %s", err))
	} else {
		templateData.Flavours = docData
	}

	var readmeTemplate template.Template
	if parsedTemplate, err := template.New("README.md.gotmpl").Funcs(sprig.FuncMap()).ParseFiles("README.md.gotmpl"); err != nil {
		panic(fmt.Sprintf("Error parsing README.md.gotmpl: %s", err))
	} else {
		readmeTemplate = *parsedTemplate
	}

	var readmeFile *os.File
	if file, err := os.OpenFile("README.md", os.O_WRONLY|os.O_CREATE, 0666); err != nil {
		panic(fmt.Sprintf("Can not open README.md: %s", err))
	} else {
		readmeFile = file
	}

	defer readmeFile.Close()

	if err := readmeTemplate.Execute(readmeFile, templateData); err != nil {
		panic(fmt.Sprintf("Can not render template to README.md: %s", err))
	}
}