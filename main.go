package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kpango/glg"
	"github.com/teamnameis/be/bone"
	"google.golang.org/grpc"
)

var (
	clothesMap sync.Map

	ml      = os.Getenv("ML_PORT")
	httick = time.NewTicker(time.Millisecond * 200)
)

const (
	grpcport = "0.0.0.0:5678"
	port     = "0.0.0.0:1234"
)

type Request struct {
	ID   int32  `json:"id"`
	Data string `json:"data"`
}

type MorphResponse struct {
	Data []byte `json:"data"`
}

type MorphRequest struct {
	User    []byte `json:"user"`
	Clothes []byte `json:"clothes"`
}

// server is used to implement helloworld.GreeterServer.
type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) Send(stream Overlay_SendServer) error {
	first := true
	tick := time.NewTicker(time.Millisecond * 200)
	var clothes []byte
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			frame, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			if frame != nil {
				glg.Info(frame.GetId())
				err = func() error {
					select {
					case <-tick.C:
						clothes, err = morph(frame.GetId(), frame.GetData())
						if err != nil {
							return stream.Send(frame)
						}
					default:
						if first {
							first = false
							clothes, err = morph(frame.GetId(), frame.GetData())
							if err != nil {
								return stream.Send(frame)
							}
						}
					}
					res, err := overlay(frame.Data, clothes)
					if err != nil {
						return stream.Send(frame)
					}
					frame.Data = res
					return stream.Send(frame)
				}()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func overlay(person, clothes []byte) ([]byte, error) {
	first, err := jpeg.Decode(bytes.NewBuffer(person))
	if err != nil {
		return nil, err
	}

	second, err := png.Decode(bytes.NewBuffer(clothes))
	if err != nil {
		return nil, err
	}

	offset := image.Pt(0, 0)
	b := first.Bounds()
	image3 := image.NewRGBA(b)
	draw.Draw(image3, b, first, image.ZP, draw.Src)
	draw.Draw(image3, second.Bounds().Add(offset), second, image.ZP, draw.Over)
	buf := new(bytes.Buffer)
	err = png.Encode(buf, image3)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func morph(id int32, user []byte) ([]byte, error) {
	conn, err := grpc.Dial(ml, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	data, err := bone.NewMLClient(conn).Morph(context.Background(), &bone.Frame{
		Id:   id,
		Data: user,
	})
	if err != nil {
		return nil, err
	}

	return data.GetData(), nil
}

func main() {

	var (
		clothes []byte
		first   = true
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := new(Request)
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()

		if data != nil {
			glg.Info(data.ID)
			img, err := base64.StdEncoding.DecodeString(data.Data)
			if err != nil {
				json.NewEncoder(w).Encode(data)
				return
			}
			select {
			case <-httick.C:
				clothes, err = morph(data.ID, img)
				if err != nil {
					json.NewEncoder(w).Encode(data)
					return
				}
			default:
				if first {
					first = false
					clothes, err = morph(data.ID, img)
					if err != nil {
						json.NewEncoder(w).Encode(data)
						return
					}
				}
			}
			res, err := overlay(img, clothes)
			if err != nil {
				json.NewEncoder(w).Encode(data)
				return
			}
			json.NewEncoder(w).Encode(Request{
				Data: base64.StdEncoding.EncodeToString(res),
			})
		}
	})
	lis, err := net.Listen("tcp", grpcport)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterOverlayServer(s, &server{})
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	http.ListenAndServe(port, nil)
}
