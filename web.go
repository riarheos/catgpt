package main

import (
	"context"
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type IndexData struct {
	UploadStatus string
	UploadError  error
	Photos       []string
	Error        error
}

var (
	indexTpl = template.Must(template.ParseFiles("tpl/index.html"))
)

func uploadAction(id *IndexData, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		id.UploadError = err
		return
	}
	defer file.Close()
	var img image.Image
	img, err = defaultGPT.EnsureIsImage(file)
	if err != nil {
		id.UploadError = err
		return
	}
	var enhanced io.Reader
	enhanced, err = defaultGPT.Enhance(img)
	name := fmt.Sprintf("%s.jpg", time.Now().String())
	err = defaultGPT.Put(r.Context(), name, enhanced)
	if err != nil {
		id.UploadError = err
	} else {
		id.UploadStatus = fmt.Sprintf("File %s uploaded", name)
	}
}

func listAction(id *IndexData, r *http.Request) {
	photos, err := defaultGPT.List(r.Context())
	if err != nil {
		id.Error = err
	} else {
		id.Photos = photos
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	id := &IndexData{}
	uploadAction(id, r)
	listAction(id, r)
	indexTpl.Execute(w, id)
}

func show(w http.ResponseWriter, r *http.Request) {
	photo, err := defaultGPT.Get(r.Context(), r.URL.Query().Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer photo.Close()
	buf, err := io.ReadAll(photo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "image/jpg")
	w.Write(buf)
}

func ping(w http.ResponseWriter, r *http.Request) {
	if isReady.Load() {
		w.WriteHeader(http.StatusOK)
	}
	w.WriteHeader(http.StatusTeapot)
}

var (
	readyGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "service_ready",
			Help: "1 if ready, 0 if not",
		})
	responseCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_response_count",
			Help: "by handler and code",
		},
		[]string{"code", "handler", "method"})
)

func serve(ctx context.Context, public string, private string) {
	indexChain := promhttp.InstrumentHandlerCounter(
		responseCounterVec.MustCurryWith(prometheus.Labels{"handler": "/"}),
		http.HandlerFunc(index))
	showChain := promhttp.InstrumentHandlerCounter(
		responseCounterVec.MustCurryWith(prometheus.Labels{"handler": "/show"}),
		http.HandlerFunc(show))
	http.HandleFunc("/", indexChain)
	http.HandleFunc("/show", showChain)
	http.HandleFunc("/ping", ping)

	prometheus.MustRegister(readyGauge, responseCounterVec)

	go func() {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		isReady.Store(true)
		readyGauge.Set(1)
		for {
			time.Sleep(time.Duration(r.Intn(60)) * time.Second)
			isReady.Store(false)
			readyGauge.Set(0)
			time.Sleep(10 * time.Second)
			isReady.Store(true)
			readyGauge.Set(1)
		}
	}()

	go func() {
		log.Fatal(http.ListenAndServe(private, promhttp.Handler()))
	}()
	log.Fatal(http.ListenAndServe(public, nil))
}
