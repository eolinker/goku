package grpc_proxy_rewrite

import (
	"strings"

	grpc_context "github.com/eolinker/eosc/eocontext/grpc-context"

	"github.com/eolinker/apinto/drivers"
	"github.com/eolinker/eosc"
	"github.com/eolinker/eosc/eocontext"
	http_service "github.com/eolinker/eosc/eocontext/http-context"
)

var _ eocontext.IFilter = (*ProxyRewrite)(nil)
var _ grpc_context.GrpcFilter = (*ProxyRewrite)(nil)

var (
	regexpErrInfo   = `[plugin proxy-rewrite2 config err] Compile regexp fail. err regexp: %s `
	notMatchErrInfo = `[plugin proxy-rewrite2 err] Proxy path rewrite fail. Request path can't match any rewrite-path. request path: %s `
)

type ProxyRewrite struct {
	drivers.WorkerBase

	host            string
	headers         map[string]string
	tls             bool
	skipCertificate bool
}

func (p *ProxyRewrite) DoFilter(ctx eocontext.EoContext, next eocontext.IChain) (err error) {
	return grpc_context.DoGrpcFilter(p, ctx, next)
}

func (p *ProxyRewrite) DoGrpcFilter(ctx grpc_context.IGrpcContext, next eocontext.IChain) (err error) {
	if p.host != "" {
		ctx.Proxy().SetHost(p.host)
	}
	ctx.EnableTls(p.tls)
	ctx.InsecureCertificateVerify(p.skipCertificate)
	for key, value := range p.headers {
		ctx.Proxy().Headers().Set(key, value)
	}
	if next != nil {
		return next.DoChain(ctx)
	}
	return nil
}

func (p *ProxyRewrite) Start() error {
	return nil
}

func (p *ProxyRewrite) Reset(v interface{}, workers map[eosc.RequireId]eosc.IWorker) error {
	conf, err := check(v)
	if err != nil {
		return err
	}
	p.skipCertificate = conf.SkipCertificate
	p.headers = conf.Headers
	p.tls = conf.Tls
	p.host = strings.TrimSpace(conf.Authority)
	return nil
}

func (p *ProxyRewrite) Stop() error {
	return nil
}

func (p *ProxyRewrite) Destroy() {

	p.headers = nil
}

func (p *ProxyRewrite) CheckSkill(skill string) bool {
	return http_service.FilterSkillName == skill
}
