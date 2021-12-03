package templates

import (
	"github.com/Nitecon/tradedesk/cache"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

//Page holds various attributes that are made available for use on templates
type Page struct {
	Content        interface{}
	BodyClass      string
	FooterScripts  []string
	Request        *http.Request
	Response       http.ResponseWriter
	SiteTitle      string
	PageTitle      string
	CurDate        time.Time
	Referrer       string
	HasPageJS      bool
	PageJSFileName string
	User           string
	SiteAuthor     string
	Params         httprouter.Params
	UserHash       string
	UserCache      *cache.CachedUser
}

func (p *Page) AddScript(s string) {
	p.FooterScripts = append(p.FooterScripts, s)
}
