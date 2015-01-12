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
package config

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/goanywhere/x/env"
)

var (
	once     sync.Once
	settings *config
)

type config struct {
	Root   string
	Debug  bool
	Secret string

	Host string
	Port int

	Templates string
}

// Settings returns a singleton settings access point.
func Settings() *config {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to retrieve project root: %v", err)
	}

	once.Do(func() {
		settings = new(config)
		settings.Debug = true
		settings.Host = "localhost"
		settings.Port = 5000

		settings.Templates = "templates"
		settings.Root, _ = filepath.Abs(cwd)

		env.Load(filepath.Join(settings.Root, ".env"))
		env.Dump(settings)
	})
	return settings
}