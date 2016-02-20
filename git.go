package triton

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

const (
	gitPathInfoRefs = "info/refs"

	gitServiceQuery               = "service"
	gitUploadPack                 = "git-upload-pack"
	gitUploadPackResult           = "application/x-git-upload-pack-result"
	gitUploadPackAdvertisement    = "application/x-git-upload-pack-advertisement"
	gitCommand                    = "git"
	gitUploadPackCommand          = "upload-pack"
	gitStatelessOption            = "--stateless-rpc"
	gitAdvertiseOption            = "--advertise-refs"
	gitServiceAdvertisementHeader = "# service="
	gitPacketFlush                = "0000"

	httpHeaderContentType     = "Content-Type"
	httpHeaderContentEncoding = "Content-Encoding"
	httpHeaderExpires         = "Expires"
	httpHeaderPragma          = "Pragma"
	httpHeaderCacheControl    = "Cache-Control"

	gzipEncoding   = "gzip"
	neverExpire    = "Fri, 01 Jan 1980 00:00:00 GMT"
	noCachePragma  = "no-cache"
	noCacheControl = "no-cache, max-age=0, must-revalidate"
)

func serveGitRequest(wr http.ResponseWriter, req *http.Request, path string) {
	// TODO: Metrics
	fmt.Println("serveGitRequest", req.RequestURI)
	serviceType := req.URL.Query().Get(gitServiceQuery)
	if gitUploadPack == serviceType {
		gitUploadPackHandler(wr, req, path)
	} else {
		wr.WriteHeader(http.StatusBadRequest)
	}
}

func serveInfoRefs(wr http.ResponseWriter, req *http.Request, path string) {
	fmt.Println("serveInfoRefs")
	packCmd := exec.Command(gitCommand, gitUploadPackCommand, gitStatelessOption, gitAdvertiseOption, path)
	packCmd.Dir = path
	if refs, err := packCmd.Output(); err != nil {
		// TODO
		fmt.Println("packCmd", err)
	} else {
		noCaching(wr)
		wr.Header().Set(httpHeaderContentType, gitUploadPackAdvertisement)
		wr.WriteHeader(http.StatusOK)
		wr.Write(gitPacketString(gitServiceAdvertisementHeader + gitUploadPackCommand + "\n"))
		wr.Write([]byte(gitPacketFlush))
		wr.Write(refs)
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

func gitPacketString(str string) []byte {
	strLen := int64(len(str)) + 4
	octalLen := strconv.FormatInt(strLen, 16)
	padding := 4 - len(octalLen)%4
	octalLen = strings.Repeat("0", padding) + octalLen
	return []byte(octalLen + str)
}

func noCaching(wr http.ResponseWriter) {
	wr.Header().Set(httpHeaderExpires, neverExpire)
	wr.Header().Set(httpHeaderPragma, noCachePragma)
	wr.Header().Set(httpHeaderCacheControl, noCacheControl)
}
