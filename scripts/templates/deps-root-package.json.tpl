{
  "name": "{{npmScope}}/{{depsPackageName}}",
  "version": "{{version}}",
  "description": "MediaGo auxiliary dependency binaries",
  "main": "install.js",
  "scripts": {
    "postinstall": "node install.js"
  },
  "optionalDependencies": {{optionalDependencies}},
  "files": ["install.js", "{{packageBinDir}}"],
  "keywords": ["mediago", "dependencies", "downloaders"],
  "license": "ISC",
  "repository": {
    "type": "git",
    "url": "https://github.com/mediago/mediago-core"
  }
}
