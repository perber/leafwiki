import autohotkey from 'highlight.js/lib/languages/autohotkey'
import bash from 'highlight.js/lib/languages/bash'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import http from 'highlight.js/lib/languages/http'
import nginx from 'highlight.js/lib/languages/nginx'
import nix from 'highlight.js/lib/languages/nix'
import powershell from 'highlight.js/lib/languages/powershell'
import protobuf from 'highlight.js/lib/languages/protobuf'
import shell from 'highlight.js/lib/languages/shell'
import { common } from 'lowlight'

export const syntaxHighlightLanguages = {
  ...common,
  autohotkey,
  bash,
  sh: bash,
  shell,
  console: shell,
  shellsession: shell,
  dockerfile,
  http,
  nginx,
  nix,
  powershell,
  protobuf,
}
