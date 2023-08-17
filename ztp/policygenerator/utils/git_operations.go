package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"sigs.k8s.io/kustomize/pkg/git"
)

var absolutePath, filePath string

var CachedUrl []string
var repo *git.RepoSpec

const (
	writeFileOutput = "output.txt"
	writeClonedInfo = "/tmp/cloned.txt"
	notClone        = "/notCloned"
	httpScheme      = "http://"
	httpsScheme     = "https://"
	rawURL          = "https://raw.githubusercontent.com"
	gitHubRepo      = "github"
	gitLabRepo      = "gitlab"
	prefix          = "./"
)

func ReadFromSourceCRPath(fileName string, crPath *[]string) ([]byte, error) {
	var (
		fileByte []byte
		err      error
	)

	for i, CRpath := range *crPath {
		switch {
		case strings.Contains(CRpath, httpScheme) || strings.Contains(CRpath, httpsScheme):
			if !strings.Contains(CRpath, gitHubRepo) && !strings.Contains(CRpath, gitLabRepo) {
				filePath = CRpath + "/" + fileName

			} else {
				// transformURL func can't be called since rawcontent
				// is not available for private repos :-(
				// so we have to clone remote repos!
				if !Contains(CachedUrl, CRpath) {
					absolutePath, repo, err = CacheContent(CRpath, writeClonedInfo)
					if err != nil {
						log.Println(err)
						return fileByte, err
					}
					CachedUrl = append(CachedUrl, CRpath)
				}
				filePath = absolutePath + "/" + fileName
			}

			switch {

			case strings.Contains(filePath, httpScheme) || strings.Contains(filePath, httpsScheme):
				fileByte, err = ReadContentRemote(filePath)

			default:
				fileByte, err = ReadFile(filePath)

			}

			if fileByte != nil && err == nil {
				WriteFile(fmt.Sprintf("- %s Read from %s", fileName, filePath), writeFileOutput)
				return fileByte, nil

			} else if err != nil && i == (len(*crPath)-1) {
				return nil, errors.New(fileName + " is not found both in any SourceCR path")
			}

		case strings.HasPrefix(CRpath, prefix):
			localDir, err := os.Executable()
			if err != nil {
				return nil, err
			}
			dir := filepath.Dir(localDir)
			p := dir + "/" + CRpath + "/" + fileName
			fileByte, err := os.ReadFile(p)
			if err == nil {
				WriteFile(fmt.Sprintf("- %s found in path %s", fileName, p), writeFileOutput)
				return fileByte, nil
			}
			if err != nil && i == (len(*crPath)-1) {
				return nil, errors.New(fileName + " is not found both in any SourceCR path")
			}
		}
	}

	return fileByte, err

}

func isCloned(fileName, url, rlPath string) bool {

	check_file, err := os.Stat(fileName)
	if err != nil {
		return false
	}

	if check_file.Size() == 0 {
		return false
	}

	var fileString []byte
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND, 0644)

	if err != nil {
		return false
	}
	defer file.Close()

	fileString, err = os.ReadFile(fileName)

	testString := strings.Split(string(fileString), ",")

	for _, v := range testString {
		if v == url || strings.Contains(v, rlPath) {
			return true
		}
	}
	return false
}

func CacheContent(url, filePath string) (string, *git.RepoSpec, error) {

	var sourceCRLocation string

	repo, err := git.NewRepoSpecFromUrl(url)
	if err != nil {
		log.Println(err)
		return sourceCRLocation, repo, err
	}

	relativePath := reflect.ValueOf(repo).Elem().FieldByName("path")
	cloneDirInfo := reflect.ValueOf(repo).Elem().FieldByName("cloneDir")
	cloned := isCloned(filePath, url, relativePath.String())

	if !cloned {
		err = git.ClonerUsingGitExec(repo)
		if err != nil {
			log.Println(err)
			return sourceCRLocation, repo, err
		}
	}

	sourceCRLocation = repo.AbsPath()
	if cloneDirInfo.String() != notClone {
		writeString := url + "," + sourceCRLocation
		WriteFile(writeString, filePath)
	}

	return sourceCRLocation, repo, nil
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// need to improve to create a http client to use connection pool
// to avoid tls handshake and tcp checking.
func ReadContentRemote(link string) ([]byte, error) {

	var content []byte
	resp, err := http.Get(link)
	if err != nil {
		log.Printf("http error ** %s **\n", err)
		return content, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return content, nil
	}

	defer resp.Body.Close()

	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Reading error from URL: ** %s **\n", err)
		return content, err
	}

	return content, nil
}

func WriteFile(content, path string) {

	fileWrite, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = fmt.Fprintln(fileWrite, content)
	if err != nil {
		log.Println(err)
		fileWrite.Close()
		return
	}
	err = fileWrite.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func ReadFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

func transformURL(url, fileName string) (string, error) {

	var newURL string
	repo, err := git.NewRepoSpecFromUrl(url)
	if err != nil {
		return newURL, err
	}
	//	fmt.Printf("hostname:  %+v\n", testString)
	hostname := reflect.ValueOf(repo).Elem().FieldByName("host")
	repoName := reflect.ValueOf(repo).Elem().FieldByName("orgRepo")
	branch := reflect.ValueOf(repo).Elem().FieldByName("ref")
	repoPath := reflect.ValueOf(repo).Elem().FieldByName("path")

	//	test := hostname.String()

	switch {
	case strings.Contains(hostname.String(), gitHubRepo):
		newURL = rawURL + "/" + repoName.String() + "/" + branch.String() + "/" + repoPath.String() + "/" + fileName

	case strings.Contains(hostname.String(), gitLabRepo):
		newURL = hostname.String() + repoName.String() + "/-/raw/" + branch.String() + "/" + repoPath.String() + "/" + fileName

	default:
		newURL = url + "/" + fileName

	}

	return newURL, nil
}

/*
 Need to remove writeClonedInfo file and the cloned directories which ar
 present under /tmp directory.
 - The cloned directory info exists inside writeClonedInfo file
 - we have to look at that file and do cleanup
 - currently the InitiatePolicyGen func() gets triggered x times the PGT file
 - Therefore, it is hard to cleanup this file
 - Need to find a workaround.
*/
// func (fHandler *FilesHandler) RemoveDir(clonedDir []string) {
// 	for _, v := range clonedDir {
// 		err := os.RemoveAll(v)
// 		if err != nil {
// 			log.Printf("Couldn't clean the repo, err: %v\n", err)
// 		}
// 	}
// }
