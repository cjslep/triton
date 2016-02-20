package triton

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"os/exec"
)

const (
	gitServiceQuery      = "service"
	gitUploadPack        = "git-upload-pack"
	gitUploadPackResult  = "application/x-git-upload-pack-result"
	gitCommand           = "git"
	gitUploadPackCommand = "upload-pack"
	gitStatelessOption   = "--stateless-rpc"

	httpHeaderContentType     = "Content-Type"
	httpHeaderContentEncoding = "Content-Encoding"

	gzipEncoding = "gzip"
)

func serveGitRequest(wr http.ResponseWriter, req *http.Request, path string) {
	// TODO: Metrics
	fmt.Println("serveGitRequest")
	serviceType := req.URL.Query().Get(gitServiceQuery)
	if gitUploadPack == serviceType {
		gitUploadPackHandler(wr, req, path)
	} else {
		wr.WriteHeader(http.StatusBadRequest)
	}
}

func gitUploadPackHandler(wr http.ResponseWriter, req *http.Request, path string) {
	body := req.Body
	if gzipEncoding == req.Header.Get(httpHeaderContentEncoding) {
		if gzipBody, err := gzip.NewReader(body); err != nil {
			// TODO
			fmt.Println("gzip", err)
			return
		} else {
			body = gzipBody
		}
	}
	wr.Header().Set(httpHeaderContentType, gitUploadPackResult)
	packCmd := exec.Command(gitCommand, gitUploadPackCommand, gitStatelessOption, path)
	packCmd.Dir = path
	packCmd.Stdin = body
	packCmd.Stdout = wr
	if err := packCmd.Run(); err != nil {
		// TODO
		fmt.Println("packCmd", err)
	}
}
