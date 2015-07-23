package utils

import (
	"io/ioutil"
	"os"

	"github.com/kopeio/kope/chained"
)

func ReadFileIfExists(path string) ([]byte, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, chained.Error(err, "error reading file", path)
	}
	return data, nil
}
