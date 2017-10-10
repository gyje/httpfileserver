/*
来源:https://github.com/Alexendoo/serve
有历史记录good
USAGE:
   serve [OPTION]... [DIR]...
OPTIONS:
       --host     --  bind to host (default: localhost)
   -i, --index    --  serve all paths to index if file not found
       --no-list  --  disable directory listings
   -p, --port     --  bind to port (default: 8080)
   -v, --verbose  --  display requests and responses
*/
package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var (
	host     string
	port     string
	index    string
	noList   bool
	verbose  bool
	version  = "HEAD"
	htmlTmpl = template.Must(template.New("html").Parse(html))
)

const (
	html = `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<style>
		body {
			font-size: 14px;
			font-family: consolas, "Liberation Mono", "DejaVu Sans Mono", Menlo, monospace;
		}
		a {
			display: block;
			color: blue;
			text-decoration: none;
		}
		a:hover {
			background-color: #f3f3f3;
		}
		.req-path {
			color: #bbb;
		}
	</style>
</head>
<body>
	{{range .}}
		<h3>
			<span class="local-path">{{.LocalPath}}</span><span class="req-path">{{.RequestPath}}</span>
		</h3>
		{{range .Entries}}
			<a class="entry{{if .IsDir}} dir{{end}}" href="{{.Link}}">{{.Name}}{{if .IsDir}}/{{end}}</a>
		{{end}}
	{{end}}
</body>
`
	usage = `
NAME:
   Serve - HTTP server for files spanning multiple directories

USAGE:
   %s [OPTION]... [DIR]...

VERSION:
   %s

OPTIONS:
       --host     --  bind to host (default: localhost)
   -i, --index    --  serve all paths to index if file not found
       --no-list  --  disable directory listings
   -p, --port     --  bind to port (default: 8080)
   -v, --verbose  --  display requests and responses
`
)

func main() {
	// handle interrupts (0 exit on ctrl + c)
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(0)
	}()

	// Log just the timestamp + message
	log.SetFlags(log.Ltime)

	flags := getFlags()
	serve(flags)
}

// getFlags returns the command line flags passed to the serve binary
func getFlags() *flag.FlagSet {
	flags := flag.NewFlagSet("flags", flag.ContinueOnError)
	flags.Usage = func() {
		usageName := filepath.Base(os.Args[0])
		fmt.Printf(usage, usageName, version)
	}
	flags.StringVar(&port, "port", "80", "")
	flags.StringVar(&port, "p", "80", "")
	flags.StringVar(&host, "host", "0.0.0.0", "")
	flags.StringVar(&index, "index", "", "")
	flags.StringVar(&index, "i", "", "")
	flags.BoolVar(&noList, "no-list", false, "")
	flags.BoolVar(&verbose, "verbose", true, "")
	flags.BoolVar(&verbose, "v", true, "")
	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		os.Exit(0)
	}
	if err != nil {
		os.Exit(1)
	}
	return flags
}

func serve(flags *flag.FlagSet) {
	dirs := make([]string, flags.NArg())
	for i := range dirs {
		dirs[i] = flags.Arg(i)
	}
	if len(dirs) == 0 {
		// serve from the current directory
		dirs = []string{"."}
	}
	http.HandleFunc("/", makeHandler(dirs))
	address := net.JoinHostPort(host, port)
	data4:="starting on: http://"+address+"\r\n"
	wlog(data4)
	log.Printf("starting on: http://%s", address)
	log.Fatal(http.ListenAndServe(address, nil))
}

func makeHandler(dirs []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		server := fmt.Sprintf("serve/%s", version)
		w.Header().Set("Server", server)
		if !validRequest(r) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			log.Printf("invalid path: %s", r.URL.Path)
			return
		}
		if tryFiles(w, r, dirs) {
			return
		}
		if len(index) > 0 && staticIndex(w, r) {
			return
		}
		if !noList && tryDirs(w, r, dirs) {
			return
		}
		http.NotFound(w, r)
	}
}

func wlog(data string){
	f, error := os.OpenFile("./run.log", 	os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if error != nil {
        fmt.Println(error)
    }
	//写入字符串
	//buf:=[]byte(data)
	now := time.Now()
	formatNow := now.Format("2006-01-02 15:04:05")
	newdata:=formatNow+"\t"+data
	f.WriteString(newdata)
	f.Close()
}
func timenow()(string){
	now := time.Now()
	formatNow := now.Format("2006-01-02 15:04:05")
	return formatNow
}

func logRequest(r *http.Request) {
	if !verbose {
		return
	}
	data := r.RemoteAddr+" -> "+r.Method+"\t"+r.RequestURI+"\t"+r.Proto+"\r\n"
	wlog(data)
	log.Printf("%s → %s %s %s", r.RemoteAddr, r.Method, r.RequestURI, r.Proto)
	/*f, error := os.OpenFile("./run.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if error != nil {
        fmt.Println(error)
    }*/
	//写入字符串
	//buf:=[]byte(data)
	//f.WriteString(data)
	//f.Close()
}

// validRequest returns false if the request is invalid: Contains ".."
func validRequest(r *http.Request) bool {
	if !strings.Contains(r.URL.Path, "..") {
		return true
	}
	for _, field := range strings.FieldsFunc(r.URL.Path, isSlashRune) {
		if field == ".." {
			return false
		}
	}
	return true
}

func isSlashRune(r rune) bool { return r == '/' || r == '\\' }

func tryFiles(w http.ResponseWriter, r *http.Request, dirs []string) bool {
	for _, dir := range dirs {
		filePath := filepath.Join(dir, r.URL.Path)
		indexPath := filepath.Join(filePath, "index.html")
		if tryFile(w, r, filePath) || tryFile(w, r, indexPath) {
			return true
		}
	}
	return false
}

// tryFile attempts to serve a file at filePath to the provided ResponseWriter
func tryFile(w http.ResponseWriter, r *http.Request, filePath string) bool {
	stat, statErr := os.Stat(filePath)
	if statErr != nil || stat.IsDir() {
		return false
	}
	file, fileErr := os.Open(filePath)
	defer file.Close()
	if fileErr != nil {
		return false
	}
	if verbose {
		filename, _ := filepath.Abs(filePath)
		data2:=r.RemoteAddr+" <- "+filename+"\r\n"
		wlog(data2)
		log.Printf("%s ← %s", r.RemoteAddr, filename)
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	return true
}

// staticIndex will attempt to serve the globally defined index file
func staticIndex(w http.ResponseWriter, r *http.Request) bool {
	file, fileErr := os.Open(index)
	defer file.Close()
	stat, statErr := os.Stat(index)
	if fileErr != nil || statErr != nil {
		log.Println(fileErr)
		return false
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	return true
}

type dirList struct {
	LocalPath   string
	RequestPath string
	Entries     []entry
}

type entry struct {
	Name  string
	Link  string
	IsDir bool
}

// tryDirs will generate directory listings for any available directories,
// providing multiple in the case that there are several matching directories
//
// Example: `serve dir1 dir2` would list directory entries dir1 containing file1,
// and dir2 containing file2 and file3
// .
// ├── dir1
// │   └── file1
// └── dir2
//     ├── file2
//     └── file3
func tryDirs(w http.ResponseWriter, r *http.Request, dirs []string) bool {
	dirLists := []dirList{}
	found := false
	for _, dir := range dirs {
		dirPath := filepath.Join(dir, r.URL.Path)
		dirInfo, err := ioutil.ReadDir(dirPath)
		if err != nil {
			continue
		}
		entries := []entry{
			{
				Name:  "..",
				Link:  path.Join(r.URL.Path, ".."),
				IsDir: true,
			},
		}
		for _, file := range dirInfo {
			entries = append(entries, entry{
				Name:  file.Name(),
				Link:  path.Join(r.URL.Path, file.Name()),
				IsDir: file.IsDir(),
			})
		}
		dirLists = append(dirLists, dirList{
			LocalPath:   dir,
			RequestPath: r.URL.Path,
			Entries:     entries,
		})
		found = true
	}
	if found {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		logDirLists(r, dirLists)
		htmlTmpl.Execute(w, dirLists)
	}
	return found
}

func logDirLists(r *http.Request, dirLists []dirList) {
	if !verbose {
		return
	}
	output := ""
	for _, dir := range dirLists {
		output += dir.LocalPath + "/, "
	}
	data3:=r.RemoteAddr+" <- "+output[:len(output)-2]+"\r\n"
	wlog(data3)
	log.Printf("%s ← %s", r.RemoteAddr, output[:len(output)-2])
}