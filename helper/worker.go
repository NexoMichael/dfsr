package helper

import (
	"context"

	"github.com/Jeffail/tunny"
	"github.com/go-ole/go-ole"
	"gopkg.in/dfsr.v0/callstat"
	"gopkg.in/dfsr.v0/versionvector"
)

type vectorWorkPool struct {
	p *tunny.WorkPool
}

type vectorJob struct {
	ctx   context.Context
	group ole.GUID
}

func newVectorWorkPool(numWorkers uint, r Reporter) (pool *vectorWorkPool, err error) {
	if numWorkers == 0 {
		return nil, ErrZeroWorkers
	}
	workers := make([]tunny.TunnyWorker, 0, numWorkers)
	for i := uint(0); i < numWorkers; i++ {
		workers = append(workers, &vectorWorker{r: r})
	}
	p, err := tunny.CreateCustomPool(workers).Open()
	if err != nil {
		return
	}
	return &vectorWorkPool{p: p}, nil
}

func (vwp *vectorWorkPool) Vector(ctx context.Context, group ole.GUID) (vector *versionvector.Vector, call callstat.Call, err error) {
	v, err := vwp.p.SendWork(vectorJob{ctx: ctx, group: group})
	if err != nil {
		return
	}

	result, ok := v.(*vectorWorkResult)
	if !ok {
		panic("invalid work result")
	}

	return result.Vector, result.Call, result.Err
}

func (vwp *vectorWorkPool) Close() {
	vwp.p.Close()
}

type vectorWorkResult struct {
	Vector *versionvector.Vector
	Call   callstat.Call
	Err    error
}

type vectorWorker struct {
	r Reporter
}

func (vw *vectorWorker) TunnyReady() bool {
	return true
}

func (vw *vectorWorker) TunnyJob(data interface{}) interface{} {
	job, ok := data.(vectorJob)
	if ok {
		var result vectorWorkResult
		result.Vector, result.Call, result.Err = vw.r.Vector(job.ctx, job.group)
		return &result
	}
	return nil
}
