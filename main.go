package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

const (
	port = "0.0.0.0:1234"
	ml   = "https://127.0.0.1:5000/"
)

type MorphResponse struct {
	Data []byte `json:"data"`
}

type MorphRequest struct {
	UBones  *Bones `json:"user_bones"`
	CBones  *Bones `json:"clothes_bones"`
	Clothes []byte `json:"clothes"`
}

type Edge struct {
	Score float64 `json:"score"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type Bone []Edge

type Bones []Bone

// server is used to implement helloworld.GreeterServer.
type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) Send(stream Overlay_SendServer) error {
	first := true
	tick := time.NewTicker(time.Millisecond * 200)
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			flame, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			if flame != nil {
				select {
				case <-tick.C:
				default:
					if first {
						first = false
						bones, err = bone(flame.Data)
						if err != nil {
							return err
						}
					}
				}
				err = stream.Send(flame)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func overlay(img1, img2 []byte) ([]byte, error) {
	first, err := png.Decode(bytes.NewBuffer(img1))
	if err != nil {
		log.Fatalf("failed to decode: %s", err)
	}

	second, err := png.Decode(bytes.NewBuffer(img2))
	if err != nil {
		log.Fatalf("failed to decode: %s", err)
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

func morph(id int, bones *Bones) ([]byte, error) {
	var body *bytes.Buffer
	err := json.NewEncoder(body).Encode(MorphRequest{
		ID:    id,
		Bones: bones,
	})
	if err != nil {
		return nil, err
	}
	res, err := http.Post(ml, "application/json", body)
	if err != nil {
		return nil, err
	}
	var data MorphResponse
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	return data.Data, nil
}

func bone(b []byte) (bones *Bones, err error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	w, err := mw.CreateFormField("image")
	if err != nil {
		return nil, err
	}
	_, err = w.Write(b)
	if err != nil {
		return nil, err
	}
	ct := mw.FormDataContentType()
	err = mw.Close()
	if err != nil {
		return nil, err
	}
	res, err := http.Post(ml, ct, body)
	if err != nil {
		return nil, err
	}
	bones = new(Bones)
	err = json.NewDecoder(res.Body).Decode(&bones)
	if err != nil {
		return nil, err
	}
	err = res.Body.Close()
	if err != nil {
		return nil, err
	}
	return bones, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterOverlayServer(s, &server{})
	// Register reflection service on gRPC server.
	// reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
