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
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kpango/glg"
	"github.com/teamnameis/be/bone"
	"google.golang.org/grpc"
)

const (
	grpcport = "0.0.0.0:5678"
	port     = "0.0.0.0:1234"
)

var (
	clothesMap sync.Map

	disableConv = strings.ToLower(os.Getenv("CONV")) != "conv"
	mode        = os.Getenv("ROTATE")
	ml          = os.Getenv("ML_PORT")
	sec         = func() time.Duration {
		t := os.Getenv("DURATION")
		if t == "" {
			return time.Millisecond * 500
		}
		dur, err := time.ParseDuration(t)
		if err != nil {
			return time.Millisecond * 500
		}
		return dur
	}()
	httick = time.NewTicker(sec)
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
	tick := time.NewTicker(sec)
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
				user, _ := rotate(frame.GetData())
				err = func() error {
					select {
					case <-tick.C:
						clothes, err = morph(frame.GetId(), user)
						if err != nil {
							glg.Errorf("morph error \tid:%d\t%v", frame.GetId(), err)
							frame.Data = imageToByte(user)
							return stream.Send(frame)
						}
					default:
						if first {
							first = false
							clothes, err = morph(frame.GetId(), user)
							if err != nil {
								glg.Errorf("morph error \tid:%d\t%v", frame.GetId(), err)
								frame.Data = imageToByte(user)
								return stream.Send(frame)
							}
						}
					}
					res, err := overlay(user, clothes)
					if err != nil {
						glg.Errorf("overlay error \tid:%d\t%v", frame.GetId(), err)
						frame.Data = imageToByte(user)
						return stream.Send(frame)
					}
					frame.Data = res
					return stream.Send(frame)
				}()
				if err != nil {
					glg.Errorf("grpc send error \tid:%d\t%v", frame.GetId(), err)
					return err
				}
			}
		}
	}
	return nil
}

func rotate(img []byte) (image.Image, error) {
	i, err := jpeg.Decode(bytes.NewBuffer(img))
	if err != nil {
		return nil, err
	}
	if strings.ToUpper(mode) == "ROTATE" {
		glg.Debug(mode)
		return imaging.Rotate270(i), nil
	}
	return i, nil
}

func imageToByte(img image.Image) []byte {
	buf := new(bytes.Buffer)
	jpeg.Encode(buf, img, nil)
	return buf.Bytes()
}

func overlay(first image.Image, clothes []byte) ([]byte, error) {
	// first, err := jpeg.Decode(bytes.NewBuffer(person))
	// if err != nil {
	// 	return nil, err
	// }
	//
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

func morph(id int32, user image.Image) ([]byte, error) {
	conn, err := grpc.Dial(ml, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, user, nil)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	data, err := bone.NewMLClient(conn).Morph(context.Background(), &bone.Frame{
		Id:   id,
		Data: buf.Bytes(),
	})
	glg.Warnf("morph spend %v", time.Now().Sub(now))
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
			if disableConv {
				json.NewEncoder(w).Encode(data)
				return
			}
			img, err := base64.StdEncoding.DecodeString(data.Data)
			if err != nil {
				json.NewEncoder(w).Encode(data)
				return
			}
			user, _ := rotate(img)
			select {
			case <-httick.C:
				clothes, err = morph(data.ID, user)
				if err != nil {
					glg.Errorf("morph error \tid:%d\t%v", data.ID, err)
					data.Data = base64.StdEncoding.EncodeToString(imageToByte(user))
					json.NewEncoder(w).Encode(data)
					return
				}
			default:
				if first {
					first = false
					clothes, err = morph(data.ID, user)
					if err != nil {
						glg.Errorf("morph error \tid:%d\t%v", data.ID, err)
						data.Data = base64.StdEncoding.EncodeToString(imageToByte(user))
						json.NewEncoder(w).Encode(data)
						return
					}
				}
			}
			res, err := overlay(user, clothes)
			if err != nil {
				data.Data = base64.StdEncoding.EncodeToString(imageToByte(user))
				glg.Errorf("overlay error \tid:%d\t%v", data.ID, err)
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
