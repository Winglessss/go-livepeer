package main

import (
	"context"
	"flag"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/lpms"
	lpmsio "github.com/livepeer/go-livepeer/lpms/io"

	"github.com/nareix/joy4/av"
)

type StreamDB struct {
	db map[string]*lpmsio.Stream
}

type BufferDB struct {
	db map[string]*lpmsio.HLSBuffer
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	lpms := lpms.New("1936", "8000", "2436", "7936")
	streamDB := &StreamDB{db: make(map[string]*lpmsio.Stream)}
	bufferDB := &BufferDB{db: make(map[string]*lpmsio.HLSBuffer)}

	lpms.HandleRTMPPublish(
		//getStreamID
		func(reqPath string) (string, error) {
			return getStreamIDFromPath(reqPath), nil
		},
		//getStream
		func(reqPath string) (*lpmsio.Stream, error) {
			streamID := getStreamIDFromPath(reqPath)
			stream := lpmsio.NewStream(streamID)
			streamDB.db[streamID] = stream
			return stream, nil
		},
		//finishStream
		func(reqPath string) {
			delete(streamDB.db, getStreamIDFromPath(reqPath))
		})

	lpms.HandleTranscode(
		//getInStream
		func(ctx context.Context, streamID string) (*lpmsio.Stream, error) {
			if stream := streamDB.db[streamID]; stream != nil {
				return stream, nil
			}

			return nil, lpmsio.ErrNotFound
		},
		//getOutStream
		func(ctx context.Context, streamID string) (*lpmsio.Stream, error) {
			//For this example, we'll name the transcoded stream "{streamID}_tran"
			newStream := lpmsio.NewStream(streamID + "_tran")
			streamDB.db[newStream.StreamID] = newStream
			return newStream, nil
		})

	lpms.HandleHTTPPlay(
		//getHLSBuffer
		func(reqPath string) (*lpmsio.HLSBuffer, error) {
			streamID := getHLSStreamIDFromPath(reqPath)
			glog.Infof("Got HTTP Req for stream: %v", streamID)
			buffer := bufferDB.db[streamID]
			stream := streamDB.db[streamID]

			if stream == nil {
				return nil, lpmsio.ErrNotFound
			}

			if buffer == nil {
				//Create the buffer and start copying the stream into the buffer
				buffer = lpmsio.NewHLSBuffer()
				bufferDB.db[streamID] = buffer
				ec := make(chan error, 1)
				go func() { ec <- stream.ReadHLSFromStream(buffer) }()
				//May want to handle the error here
			}
			return buffer, nil

		})

	lpms.HandleRTMPPlay(
		//getStream
		func(ctx context.Context, reqPath string, dst av.MuxCloser) error {
			glog.Infof("Got req: ", reqPath)
			streamID := getStreamIDFromPath(reqPath)
			src := streamDB.db[streamID]

			if src != nil {
				src.ReadRTMPFromStream(ctx, dst)
			} else {
				glog.Error("Cannot find stream for ", streamID)
				return lpmsio.ErrNotFound
			}
			return nil
		})

	//Helper function to print out all the streams
	http.HandleFunc("/streams", func(w http.ResponseWriter, r *http.Request) {
		streams := []string{}

		for k, _ := range streamDB.db {
			streams = append(streams, k)
		}

		if len(streams) == 0 {
			w.Write([]byte("no streams"))
			return
		}
		str := strings.Join(streams, ",")
		w.Write([]byte(str))
	})

	lpms.Start()
}

func getStreamIDFromPath(reqPath string) string {
	return "test"
}

func getHLSStreamIDFromPath(reqPath string) string {
	if strings.HasSuffix(reqPath, ".m3u8") {
		return "test_tran"
	} else {
		return "test_tran"
	}
}
