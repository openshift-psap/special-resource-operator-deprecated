package specialresource

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

type assetsFromFile struct {
	name    string
	content []byte
}

func getAssetsFrom(asset string) []assetsFromFile {

	manifests := []assetsFromFile{}
	files, err := filePathWalkDir(asset)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		buffer, err := ioutil.ReadFile(file)
		if err != nil {
			panic(err)
		}
		manifests = append(manifests, assetsFromFile{path.Base(file), buffer})
	}
	return manifests
}

func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
