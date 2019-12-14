//
// Copyright (C) 2019 Vdaas.org Vald team ( kpango, kmrmt, rinx )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package grpc provides grpc server logic
package grpc

import (
	"context"
	"strconv"

	"github.com/teamnameis/be/apis/grpc/agent"
	"github.com/teamnameis/be/apis/grpc/payload"
	"github.com/teamnameis/be/internal/errors"
	"github.com/teamnameis/be/internal/net/grpc"
	"github.com/teamnameis/be/internal/net/grpc/status"
	"github.com/teamnameis/be/pkg/be/model"
	"github.com/teamnameis/be/pkg/be/service"
)

type Server be.Overlay

type server struct {
	be               service.BE
}

type errDetail struct {
	method string
	id     string
	ids    []string
}

func New(opts ...Option) Server {
	s := new(server)

	for _, opt := range append(defaultOpts, opts...) {
		opt(s)
	}
	return s
}

func (s *server) Send(ctx context.Context, req *be.Flame) (res *be.Flame, err error) {
	data, err := s.be.Overlay(req.Data)
	if err != nil{
		return nil, err
	}
	return &be.Flame{
		Data: dara,
	},nil
}
