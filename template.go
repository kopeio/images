package kope

import (
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"
)

func WriteTemplate(path string, data interface{}) error {
	tempPath, err := WriteTemplateTempFile(path, data)
	if err != nil {
		return err
	}

	err = os.Rename(tempPath, path)
	if err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("error renaming templated file: %v", err)
	}

	return nil
}

func WriteTemplateTempFile(path string, data interface{}) (string, error) {
	t := template.New("template:" + path)

	templateKey := filepath.Base(path)
	templatePath := "/templates/" + templateKey + ".template"

	templateDefinition, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("error reading template file (%s): %v", templatePath, err)
	}

	template, err := t.Parse(string(templateDefinition))
	if err != nil {
		return "", fmt.Errorf("error parsing template file (%s): %v", templatePath, err)
	}

	var buffer bytes.Buffer

	err = template.Execute(&buffer, data)
	if err != nil {
		return "", fmt.Errorf("error executing template file (%s): %v", templatePath, err)
	}

	if glog.V(4) {
		glog.Info("Writing file %s\n%s", path, string(buffer.Bytes()))
	}

	tempPath := path + "." + strconv.FormatInt(time.Now().UnixNano(), 10)

	err = ioutil.WriteFile(tempPath, buffer.Bytes(), 0777)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("error writing templated file (%s): %v", path, err)
	}

	return tempPath, nil
}
