{
  "name": "{{npmScope}}/{{corePackageName}}",
  "version": "{{version}}",
  "description": "MediaGo Player - A hybrid Go+React video player server",
  "main": "install.js",
  "scripts": {
    "postinstall": "node install.js"
  },
  "bin": {
    "{{appName}}": "bin/{{appName}}"
  },
  "optionalDependencies": {{optionalDependencies}},
  "files": ["install.js", "bin"],
  "keywords": ["video", "core", "server", "go", "react", "mediago"],
  "author": "",
  "license": "ISC",
  "repository": {
    "type": "git",
    "url": "https://github.com/mediago/mediago-core"
  }
}
