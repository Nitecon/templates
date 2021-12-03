package templates

import (
	"bytes"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Nitecon/tradedesk/config"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
)

var (
	// MyTemplates variable includes the list of all tempaltes available to the system as found by walking the filesystem.
	MyTemplates *template.Template

	templateFuncs = template.FuncMap{
		"contains": strings.Contains,
		"dump":     func(field interface{}) string { return dump(field) },
	}
	tplMap     = make(map[string]string)
	tplMapLock = new(sync.RWMutex)
)

func dump(field interface{}) string {
	buf := &bytes.Buffer{}
	spew.Fdump(buf, field)
	return buf.String()
}
func getTplFromMap(name string) (string, error) {
	tplMapLock.RLock()
	defer tplMapLock.RUnlock()
	if val, ok := tplMap[name]; ok {
		return val, nil
	}
	return "", fmt.Errorf("could not find path for template: %s", name)
}
func appendTplMap(name string, path string) {
	tplMapLock.Lock()
	defer tplMapLock.Unlock()
	tplMap[name] = path
}

// Setup creates a buffer and does a filesystem walk to add all templates to a global parent.
func Setup() {
	MyTemplates = template.New("templates")
	_, err := os.Stat(config.Get().TemplateDir)
	if os.IsNotExist(err) {
		log.Fatal().Msgf("Failed to load templates from dir (%s): %s", config.Get().TemplateDir, err.Error())
	}
	filepath.Walk(config.Get().TemplateDir, appendTemplate)
}

func appendTemplate(fp string, fi os.FileInfo, err error) (e error) {

	if err != nil {
		log.Error().Msgf("Cannot walk template directory: %s", err.Error())
	}
	if fi.IsDir() {
		return // not a file.  ignore.
	}
	matched, e := filepath.Match("*.html", fi.Name())
	if e != nil {
		log.Error().Msgf("Malformed pattern in walk: %s", err.Error())
		return // this is fatal.
	}
	if matched {
		sName := ""
		if runtime.GOOS == "windows" {
			i, j := strings.LastIndex(fp, "\\")+1, strings.LastIndex(fp, path.Ext(fp))
			sName = fp[i:j]
		} else {
			sName = path.Base(fp)
		}
		//log.Infof("Template sName: %s\n", sName)

		var extension = filepath.Ext(sName)
		var name = sName[0 : len(sName)-len(extension)]
		appendTplMap(name, fp)
		if strings.Contains(filepath.ToSlash(fp), "/inc/") {
			//log.Infof("Template Basename: %s", name)
			data, err := ioutil.ReadFile(fp)
			if err != nil {
				log.Error().Msgf("Could not read contents of template file: %s:", sName)
				return
			}

			MyTemplates.New(name).Funcs(templateFuncs).Parse(string(data))
		}
		//log.Debugf("Matched template read: %s", name)
		return
	}
	//log.Infof("FilePath: %s was not matched", fp)
	return
}

// Render is a utility function that allows the page to render by using it's render template function.
func (my *Page) Render(tpl string) {
	var scripts []string
	for i := len(my.FooterScripts) - 1; i >= 0; i-- {
		scripts = append(scripts, my.FooterScripts[i])
	}
	my.FooterScripts = scripts
	RenderTemplate(my.Response, tpl, my)
}

// RenderError is a utility function that allows the page to render an error directly.
func (my *Page) RenderError(err error) {

	var scripts []string
	for i := len(my.FooterScripts) - 1; i >= 0; i-- {
		scripts = append(scripts, my.FooterScripts[i])
	}
	my.FooterScripts = scripts
	if err.Error() == "The access token being passed has expired or is invalid." {
		my.RenderRedirect("/td/renew", true)
	} else {
		my.Content = err
		RenderTemplate(my.Response, "error", my)
	}

}

// RenderNotFound is a utility function that allows the page to render a 404 error.
func (my *Page) RenderNotFound() {
	var scripts []string
	for i := len(my.FooterScripts) - 1; i >= 0; i-- {
		scripts = append(scripts, my.FooterScripts[i])
	}
	my.FooterScripts = scripts
	my.Content = my.Request.RequestURI
	RenderTemplate(my.Response, "404", my)
}

// RenderUnauthorized is a utility function that allows the page to render a 403 error.
func (my *Page) RenderUnauthorized() {
	var scripts []string
	for i := len(my.FooterScripts) - 1; i >= 0; i-- {
		scripts = append(scripts, my.FooterScripts[i])
	}
	my.FooterScripts = scripts
	my.Content = my.Request.RequestURI
	RenderTemplate(my.Response, "403", my)
}

// RenderRedirect is a utility function that allows the page to render a redirect.
func (my *Page) RenderRedirect(location string, doOriginRedirect bool) {
	var scripts []string
	for i := len(my.FooterScripts) - 1; i >= 0; i-- {
		scripts = append(scripts, my.FooterScripts[i])
	}
	if doOriginRedirect {
		my.Content = fmt.Sprintf("%s?origin=%s", location, my.Request.RequestURI)
	} else {
		my.Content = location
	}
	my.FooterScripts = scripts
	my.Content = location
	RenderTemplate(my.Response, "redirect", my)
}

// RenderTemplate is a wrapper around template.ExecuteTemplate.
func RenderTemplate(w http.ResponseWriter, name string, data interface{}) (err error) {
	fp, err := getTplFromMap(name)
	if err != nil {
		log.Error().Msgf("Could not load template %s \n%s\n", name, err)
		return err
	}
	td, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Error().Msgf("Could not read contents of template file: %s:", fp)
		return
	}
	t, err := MyTemplates.Clone()
	if err != nil {
		log.Error().Err(err).Msg("Could not clone my templates for use")
	}
	tpl, err := t.New(name).Parse(string(td))
	if err != nil {
		log.Error().Msgf("Could not read template %s:\nError: %s\n", name, err)
	}
	err = tpl.Execute(w, data)
	if err != nil {
		log.Error().Msgf("Could not execute template %s\nError: %s\n", name, err)
		return err
	}
	return nil
	/*
		//tpl := MyTemplates.Lookup(name)
		// Ensure the template exists in the map.

		log.Debugf("Could not find template: %s", name)
		//log.Debugf("Available templates:")
		for _, tpl := range MyTemplates.Templates() {
			log.Debugf("%s", tpl.Name())
		}

		//Tpl404 := MyTemplates.Lookup
		etpl := template.New("ErrorTemplate")
		errTpl, errt := etpl.Parse(Tpl404)
		if errt != nil {
			log.Errorf("Could not parse the 404 template thus failure...: %s", err.Error())
			w.Write([]byte("Serious error occurred exiting now: " + err.Error()))
			return
		}
		err = errTpl.Execute(w, err)
		if err != nil {
			log.Errorf("Error executing template: %s", err.Error())
			w.WriteHeader(500)
			w.Write([]byte("A serious error occurred and we cannot continue!"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	*/
}

// GenerateDefaults creates a basic initial data import for use in every page.
func GenerateDefaults(p *Page) (page *Page) {
	page = p
	page.CurDate = time.Now()
	return
}

// GetBasePage will return the basic initialized page struct used by all templates.
func GetBasePage(w http.ResponseWriter, r *http.Request, params httprouter.Params, pageTitle string) *Page {
	p := GenerateDefaults(new(Page))

	scriptPath := fmt.Sprintf("static/js%s.%s", r.URL.Path, "js")
	if path.Base(scriptPath) == ".js" {
		scriptPath = fmt.Sprintf("static/js%sindex.%s", r.URL.Path, "js")
	}
	log.Debug().Msgf("Searching for a page level js file: /%s", scriptPath)
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		p.PageJSFileName = "/" + scriptPath
		p.HasPageJS = true
	}
	p.Params = params
	p.BodyClass = "sb-nav-fixed"
	p.PageTitle = pageTitle
	p.SiteTitle = "Unknown Site Title"
	p.User = "Anonymous"
	p.SiteAuthor = "Nitecon Studios LLC"

	p.Request = r
	p.Response = w
	return p
}
