/*
 * Copyright 2022 The Gremlins Authors
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package mutator_test

import (
	"errors"
	"go/token"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/go-gremlins/gremlins/configuration"
	"github.com/go-gremlins/gremlins/internal/gomodule"
	"github.com/go-gremlins/gremlins/pkg/mutant"
	"github.com/go-gremlins/gremlins/pkg/mutator/internal/workerpool"
)

var viperMutex sync.RWMutex

func init() {
	viperMutex.Lock()
	viperReset()
}

func viperSet(set map[string]any) {
	viperMutex.Lock()
	for k, v := range set {
		configuration.Set(k, v)
	}
}

func viperReset() {
	configuration.Reset()
	for _, mt := range mutant.Types {
		configuration.Set(configuration.MutantTypeEnabledKey(mt), true)
	}
	viperMutex.Unlock()
}

func loadFixture(fixture, fromPackage string) (fstest.MapFS, gomodule.GoModule, func()) {
	f, _ := os.Open(fixture)
	src, _ := io.ReadAll(f)
	filename := filenameFromFixture(fixture)
	mapFS := fstest.MapFS{
		filename: {Data: src},
	}

	return mapFS, gomodule.GoModule{
			Name:       "example.com",
			Root:       ".",
			CallingDir: fromPackage,
		}, func() {
			_ = f.Close()
		}
}

func filenameFromFixture(fix string) string {
	return strings.ReplaceAll(fix, "_go", ".go")
}

type dealerStub struct {
	t *testing.T
}

func newWdDealerStub(t *testing.T) dealerStub {
	t.Helper()

	return dealerStub{t: t}
}

func (d dealerStub) Get(_ string) (string, error) {
	return d.t.TempDir(), nil
}

func (dealerStub) Clean() {}

type executorDealerStub struct {
	gotMutants []mutant.Mutant
}

func newJobDealerStub(t *testing.T) *executorDealerStub {
	t.Helper()

	return &executorDealerStub{}
}

func (j *executorDealerStub) NewExecutor(mut mutant.Mutant, outCh chan<- mutant.Mutant, wg *sync.WaitGroup) workerpool.Executor {
	j.gotMutants = append(j.gotMutants, mut)

	return &executorStub{
		mut:   mut,
		outCh: outCh,
		wg:    wg,
	}
}

type executorStub struct {
	mut   mutant.Mutant
	outCh chan<- mutant.Mutant
	wg    *sync.WaitGroup
}

func (j *executorStub) Start(_ *workerpool.Worker) {
	j.outCh <- j.mut
	j.wg.Done()
}

type mutantStub struct {
	worDir         string
	pkg            string
	position       token.Position
	status         mutant.Status
	mutType        mutant.Type
	applyCalled    bool
	rollbackCalled bool

	hasApplyError bool
}

func (m *mutantStub) Type() mutant.Type {
	return m.mutType
}

func (m *mutantStub) SetType(mt mutant.Type) {
	m.mutType = mt
}

func (m *mutantStub) Status() mutant.Status {
	return m.status
}

func (m *mutantStub) SetStatus(s mutant.Status) {
	m.status = s
}

func (m *mutantStub) Position() token.Position {
	return m.position
}

func (*mutantStub) Pos() token.Pos {
	panic("not used in test")
}

func (m *mutantStub) Pkg() string {
	return m.pkg
}

func (m *mutantStub) SetWorkdir(w string) {
	m.worDir = w
}

func (m *mutantStub) Apply() error {
	m.applyCalled = true
	if m.hasApplyError {
		return errors.New("test error")
	}

	return nil
}

func (m *mutantStub) Rollback() error {
	m.rollbackCalled = true

	return nil
}
