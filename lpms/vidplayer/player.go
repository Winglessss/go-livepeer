package vidplayer

import (
	"context"
	"net/http"

	"strings"

	"github.com/golang/glog"
	lpmsio "github.com/livepeer/go-livepeer/lpms/io"
	"github.com/nareix/joy4/av"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

//VidPlayer is the module that handles playing video. For now we only support RTMP and HLS play.
type VidPlayer struct {
	RtmpServer *joy4rtmp.Server
}

//HandleRTMPPlay is the handler when there is a RTMP request for a video. The source should write
//into the MuxCloser. The easiest way is through avutil.Copy.
func (s *VidPlayer) HandleRTMPPlay(getStream func(ctx context.Context, reqPath string, dst av.MuxCloser) error) error {
	s.RtmpServer.HandlePlay = func(conn *joy4rtmp.Conn) {
		glog.Infof("LPMS got RTMP request @ %v", conn.URL)

		ctx := context.Background()
		c := make(chan error, 1)
		go func() { c <- getStream(ctx, conn.URL.Path, conn) }()
		select {
		case err := <-c:
			glog.Errorf("Rtmp getStream Error: %v", err)
		}
	}
	return nil
}

//HandleHTTPPlay is the handler when there is a HLA request. The source should write the raw bytes into the io.Writer,
//for either the playlist or the segment.
func (s *VidPlayer) HandleHTTPPlay(getHLSBuffer func(reqPath string) (*lpmsio.HLSBuffer, error)) error {
	http.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("LPMS got HTTP request @ %v", r.URL.Path)

		if !strings.HasSuffix(r.URL.Path, ".m3u8") && !strings.HasSuffix(r.URL.Path, ".ts") {
			http.Error(w, "LPMS only accepts HLS requests over HTTP (m3u8, ts).", 500)
		}

		ctx := context.Background()
		// c := make(chan error, 1)
		// go func() { c <- getStream(ctx, r.URL.Path, w) }()
		buffer, err := getHLSBuffer(r.URL.Path)
		if err != nil {
			glog.Errorf("Error getting HLS Buffer: %v", err)
		}

		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			pl, err := buffer.WaitAndPopPlaylist(ctx)
			if err != nil {
				glog.Errorf("Error getting HLS playlist %v: %v", r.URL.Path, err)
				return
			}
			_, err = w.Write(pl.Encode().Bytes())
			if err != nil {
				glog.Errorf("Error writting HLS playlist %v: %v", r.URL.Path, err)
				return
			}
		}

		if strings.HasSuffix(r.URL.Path, ".ts") {
			pathArr := strings.Split(r.URL.Path, "/")
			segName := pathArr[len(pathArr)-1]
			seg, err := buffer.WaitAndPopSegment(ctx, segName)
			if err != nil {
				glog.Errorf("Error getting HLS segment %v: %v", segName, err)
				return
			}
			_, err = w.Write(seg)
			if err != nil {
				glog.Errorf("Error writting HLS segment %v: %v", segName, err)
				return
			}
		}

		http.Error(w, "Cannot find HTTP video resource: "+r.URL.Path, 500)
	})
	return nil
}
