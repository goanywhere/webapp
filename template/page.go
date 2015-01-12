/* ----------------------------------------------------------------------
 *       ______      ___                         __
 *      / ____/___  /   |  ____  __  ___      __/ /_  ___  ________
 *     / / __/ __ \/ /| | / __ \/ / / / | /| / / __ \/ _ \/ ___/ _ \
 *    / /_/ / /_/ / ___ |/ / / / /_/ /| |/ |/ / / / /  __/ /  /  __/
 *    \____/\____/_/  |_/_/ /_/\__. / |__/|__/_/ /_/\___/_/   \___/
 *                            /____/
 *
 * (C) Copyright 2015 GoAnywhere (http://goanywhere.io).
 * ----------------------------------------------------------------------
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 * ----------------------------------------------------------------------*/
package template

import (
	"html/template"
	"io/ioutil"
	"log"
	"path"
	"regexp"
	"strings"
)

var (
	regexExtends = regexp.MustCompile(`{%\s+extends\s+["]([^"]*\.html)["]\s+%}`)
	regexInclude = regexp.MustCompile(`{%\s+include\s+["]([^"]*\.html)["]\s+%}`)
)

type page struct {
	Name   string  // name of the page under laoder's root path.
	loader *Loader // file loader.
}

// Ancesters finds all ancestors absolute path using jinja's syntax
// and combines them along with the page name iteself into correct order for parsing.
// tag: {% extends "layout/base.html" %}
func (self *page) ancestors() (names []string) {
	var name = self.Name
	names = append(names, name)

	for {
		// find the very first "extends" tag.
		var bits, err = ioutil.ReadFile(path.Join(self.loader.root, name))
		if err != nil {
			log.Fatalf("Failed to open template (%s): %v", name, err)
		}

		var result = regexExtends.FindSubmatch(bits)
		if result == nil {
			break
		}

		var base = string(result[1])
		if base == name {
			log.Fatalf("Template cannot extend itself (%s)", name)
		}

		names = append([]string{base}, names...) // insert the ancester into the first place.
		name = base
	}

	return
}

// Include finds all included external file sources recursively
// & replace all the "include" tags with their actual sources.
// tag: {% include "partials/header.html" %}
func (self *page) include() (source string) {
	bits, err := ioutil.ReadFile(self.path())
	if err != nil {
		log.Fatalf("Failed to open template (%s): %v", self.Name, err)
	}

	source = string(bits)
	for {
		result := regexInclude.FindAllStringSubmatch(source, -1)
		if result == nil {
			break
		}

		for _, match := range result {
			tag, name := match[0], match[1]
			if name == self.Name {
				log.Fatalf("Template cannot include itself (%s)", name)
			}
			page := self.loader.page(name)
			// reconstructs source to recursively find all included sources.
			source = strings.Replace(source, tag, page.source(), -1)
		}
	}
	return
}

// Parse constructs `template.Template` object with additional // "extends" & "include" like Jinja.
func (self *page) parse() (output *template.Template) {
	var e error
	names := self.ancestors()

	for _, name := range names {
		var tmpl *template.Template
		var page = self.loader.page(name)

		if output == nil {
			output = template.New(name).Funcs(Functions)
		}
		if name == output.Name() {
			tmpl = output
		} else {
			tmpl = output.New(name)
		}
		_, e = tmpl.Parse(page.include())
	}

	return template.Must(output, e)
}

// Path returns the abolute path of the page.
func (self *page) path() string {
	return path.Join(self.loader.root, self.Name)
}

// Source returns the plain raw source of the page.
func (self *page) source() (src string) {
	if bits, err := ioutil.ReadFile(self.path()); err == nil {
		src = string(bits)
	} else {
		log.Fatalf("Failed to open template (%s): %v", self.Name, err)
	}
	return src
}