package triton

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	gitPathInfoRefs = "info/refs"
	gitPathHead     = "HEAD"

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
	httpHeaderContentLength   = "Content-Length"
	httpHeaderLastModified    = "Last-Modified"

	gzipEncoding   = "gzip"
	plainText      = "text/plain"
	neverExpire    = "Fri, 01 Jan 1980 00:00:00 GMT"
	noCachePragma  = "no-cache"
	noCacheControl = "no-cache, max-age=0, must-revalidate"
)

func addGitHandlers(mux *http.ServeMux, gitDir string, fullPathGitDir string) {
	mux.HandleFunc(gitDir, func(wr http.ResponseWriter, req *http.Request) {
		serveGitRequest(wr, req, fullPathGitDir)
	})
	mux.HandleFunc(gitDir+gitPathInfoRefs, func(wr http.ResponseWriter, req *http.Request) {
		serveInfoRefs(wr, req, gitDir)
	})
	mux.HandleFunc(gitDir+gitPathHead, func(wr http.ResponseWriter, req *http.Request) {
		serveTextFileNoCaching(wr, req, fullPathGitDir+gitPathHead)
	})
}

func serveGitRequest(wr http.ResponseWriter, req *http.Request, path string) {
	// TODO: Metrics
	fmt.Println("serveGitRequest", req.RequestURI, path)
	serviceType := req.URL.Query().Get(gitServiceQuery)
	if gitUploadPack == serviceType {
		serveUploadPack(wr, req, path)
	} else {
		wr.WriteHeader(http.StatusBadRequest)
	}
}

func serveInfoRefs(wr http.ResponseWriter, req *http.Request, path string) {
	fmt.Println("serveInfoRefs", path)
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

func serveUploadPack(wr http.ResponseWriter, req *http.Request, path string) {
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

func serveTextFileNoCaching(wr http.ResponseWriter, req *http.Request, file string) {
	fmt.Println("serveTextFileNoCaching", file)
	noCaching(wr)
	serveFile(plainText, wr, req, file)
}

func serveFile(contentType string, wr http.ResponseWriter, req *http.Request, file string) {
	fileStats, err := os.Stat(file)
	if os.IsNotExist(err) {
		// TODO
		fmt.Println("serveFile", err)
		return
	} else if err != nil {
		// TODO
		fmt.Println("serveFile", err)
		return
	}
	wr.Header().Set(httpHeaderContentType, contentType)
	wr.Header().Set(httpHeaderContentLength, strconv.Itoa(int(fileStats.Size())))
	wr.Header().Set(httpHeaderLastModified, fileStats.ModTime().Format(http.TimeFormat))
	http.ServeFile(wr, req, file)
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
