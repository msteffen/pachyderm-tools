#!/usr/bin/python3

import datetime
import markdown
import sys
import os
from http import HTTPStatus
import http.server
# import email.utils as u
from pygments.formatters import HtmlFormatter
from urllib.parse import quote, unquote, parse_qs, urlencode
import commonmark

class RenderMDServer(http.server.BaseHTTPRequestHandler):
  def render_dir(self, params):
    wd = os.path.basename(os.getcwd())
    files = [ f for f in os.listdir() if f.endswith(".md") ]
    files.sort(reverse=True)
    print("files: {}".format(files))
    if len(files) == 0:
      msg = "no markdown files in current working dir ({})".format(wd)
      self.send_error(404, msg)
      return
    elif len(files) > 1:
      self.send_response(200)
      self.send_header("Content-Type", "text/html")
      self.send_header("Last-Modified",
          self.date_time_string(datetime.datetime.now().timestamp()))
      self.end_headers()
      self.wfile.write(b"<html><head><title>")
      self.wfile.write(b"Available Files")
      self.wfile.write(b"</title></head><body><h3>Available Files</h3><ul>")
      for f in files:
        fpath = quote(f, safe='')
        if len(params) > 0:
          fpath += "?"+urlencode(params, doseq=True)
        self.wfile.write(
          "<li/><a href={1}>{0}</a><br/>"
            .format(f, fpath)
            .encode() # Convert to binary string for write()
        )
      self.wfile.write(b"</ul></body></html>")
      return
    else:
      self.send_response(HTTPStatus.FOUND)
      newpath = "/" + quote(files[0].lstrip("/"))
      if len(params) > 0:
        newpath += "?"+urlencode(params)
      self.send_header("Location", newpath, safe='')
      self.end_headers()

  def render_file(self, filename, use_commonmark):
    # Convert .md to html
    if "." not in filename: filename += ".md"
    if not filename.endswith(".md"):
      msg = filename + " not found"
      self.send_error(404, msg)
      return
    try:
      fs = os.stat(filename)
    except FileNotFoundError:
      self.send_error(404, "{} not found".format(filename))
      return
    with open(filename, "r") as fd:
      print("fd: {}".format(fd))
      # Use browser cache if possible
      # Stolen from python SimpleHTTPServer. See:
      # https://github.com/python/cpython/blob/6bf382ac9a07f42c6f91f35a7f0df4c63e35a8a7/Lib/http/server.py#L709-L745
      # if ("If-Modified-Since" in self.headers
      #     and "If-None-Match" not in self.headers):
      #     print("If-Modified-Since: {}".format(self.headers["If-Modified-Since"]))
      #     # compare If-Modified-Since and time of last file modification
      #     try:
      #       ims = u.parsedate_to_datetime(self.headers["If-Modified-Since"])
      #     except (TypeError, IndexError, OverflowError, ValueError):
      #       # ignore ill-formed values
      #       pass
      #     else:
      #       print("ims: {}".format(ims))
      #       if ims.tzinfo is None:
      #         # obsolete format with no timezone, cf.
      #         # https://tools.ietf.org/html/rfc7231#section-7.1.1.1
      #         ims = ims.replace(tzinfo=datetime.timezone.utc)
      #       if ims.tzinfo is datetime.timezone.utc:
      #         # compare to UTC datetime of last modification
      #         last_modif = datetime.datetime.fromtimestamp(
      #           fs.st_mtime, datetime.timezone.utc)
      #         # remove microseconds, like in If-Modified-Since
      #         last_modif = last_modif.replace(microsecond=0)
      #         if last_modif <= ims:
      #           self.send_response(HTTPStatus.NOT_MODIFIED)
      #           self.end_headers()
      #           return None

      print("Status: OK")
      self.send_response(HTTPStatus.OK)
      self.send_header("Content-Type", "text/html")
      self.send_header("Last-Modified",
          self.date_time_string(fs.st_mtime))
      self.end_headers()

      # render markdown into 'html' (which won't have head or stylesheet)
      if use_commonmark:
        parser = commonmark.Parser()
        ast = parser.parse(fd.read())
        renderer = commonmark.HtmlRenderer()
        html = renderer.render(ast)
      else:
        # Built-in markdown extensions
        extensions = [ "markdown.extensions." + name for name in
           ["footnotes", "tables", "codehilite", "fenced_code", "toc"]]
        # pip3 install --user pymdown-extensions
        # https://github.com/facelessuser/pymdown-extensions
        extensions += [ "pymdownx." + name for name in
          [ "magiclink", "tilde", "tasklist"] ]
        html = markdown.markdown(fd.read(), extensions=extensions, tab_length=2)

      # Write out html output
      # Import codehilite css style sheet
      self.wfile.write(b"<html><head><style>\n")
      # pip3 install --user pygments
      self.wfile.write(HtmlFormatter().get_style_defs(".codehilite").encode())
      self.wfile.write(github_css().encode())
      self.wfile.write(b"</style></head><body class=\"markdown-body\">\n")
      self.wfile.write(html.encode())
      self.wfile.write(b"</body></html>")

  def do_GET(self):
    params = {}
    path_parts = self.path.split("?")
    if len(path_parts) > 2:
        self.send_error(500, "invalid URL, multiple '?'")
    elif len(path_parts) == 2:
        params = parse_qs(path_parts[1])
    for p in params:
      if p != "c":
        self.send_error(400, "unrecognized param: {}".format(p))
    if path_parts[0] == "/":
      return self.render_dir(params)
    else:
      return self.render_file(path_parts[0].lstrip("/"), "c" in params)

    ##########################################################
    # Specific path was requested
    ##########################################################

def main():
  port = 28000
  while port <= 30000:
    try:
      server = http.server.HTTPServer(("localhost", port), RenderMDServer)
      print("serving markdown on localhost:{}".format(port))
      server.serve_forever()
      return
    except OSError as e:
      print("port {} in use ({}), continuing...".format(port, e))
      port += 1
      continue

def github_css():
  return """
  @font-face {
  font-family: octicons-link;
  src: url(data:font/woff;charset=utf-8;base64,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) format('woff');
}

.markdown-body .octicon {
  display: inline-block;
  fill: currentColor;
  vertical-align: text-bottom;
}

.markdown-body .anchor {
  float: left;
  line-height: 1;
  margin-left: -20px;
  padding-right: 4px;
}

.markdown-body .anchor:focus {
  outline: none;
}

.markdown-body h1 .octicon-link,
.markdown-body h2 .octicon-link,
.markdown-body h3 .octicon-link,
.markdown-body h4 .octicon-link,
.markdown-body h5 .octicon-link,
.markdown-body h6 .octicon-link {
  color: #1b1f23;
  vertical-align: middle;
  visibility: hidden;
}

.markdown-body h1:hover .anchor,
.markdown-body h2:hover .anchor,
.markdown-body h3:hover .anchor,
.markdown-body h4:hover .anchor,
.markdown-body h5:hover .anchor,
.markdown-body h6:hover .anchor {
  text-decoration: none;
}

.markdown-body h1:hover .anchor .octicon-link,
.markdown-body h2:hover .anchor .octicon-link,
.markdown-body h3:hover .anchor .octicon-link,
.markdown-body h4:hover .anchor .octicon-link,
.markdown-body h5:hover .anchor .octicon-link,
.markdown-body h6:hover .anchor .octicon-link {
  visibility: visible;
}

.markdown-body {
  -ms-text-size-adjust: 100%;
  -webkit-text-size-adjust: 100%;
  color: #24292e;
  line-height: 1.5;
  font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif,Apple Color Emoji,Segoe UI Emoji,Segoe UI Symbol;
  font-size: 16px;
  line-height: 1.5;
  word-wrap: break-word;
  width: 800pt;
  margin: 30pt auto;
}

.markdown-body .pl-c {
  color: #6a737d;
}

.markdown-body .pl-c1,
.markdown-body .pl-s .pl-v {
  color: #005cc5;
}

.markdown-body .pl-e,
.markdown-body .pl-en {
  color: #6f42c1;
}

.markdown-body .pl-s .pl-s1,
.markdown-body .pl-smi {
  color: #24292e;
}

.markdown-body .pl-ent {
  color: #22863a;
}

.markdown-body .pl-k {
  color: #d73a49;
}

.markdown-body .pl-pds,
.markdown-body .pl-s,
.markdown-body .pl-s .pl-pse .pl-s1,
.markdown-body .pl-sr,
.markdown-body .pl-sr .pl-cce,
.markdown-body .pl-sr .pl-sra,
.markdown-body .pl-sr .pl-sre {
  color: #032f62;
}

.markdown-body .pl-smw,
.markdown-body .pl-v {
  color: #e36209;
}

.markdown-body .pl-bu {
  color: #b31d28;
}

.markdown-body .pl-ii {
  background-color: #b31d28;
  color: #fafbfc;
}

.markdown-body .pl-c2 {
  background-color: #d73a49;
  color: #fafbfc;
}

.markdown-body .pl-c2:before {
  content: "^M";
}

.markdown-body .pl-sr .pl-cce {
  color: #22863a;
  font-weight: 700;
}

.markdown-body .pl-ml {
  color: #735c0f;
}

.markdown-body .pl-mh,
.markdown-body .pl-mh .pl-en,
.markdown-body .pl-ms {
  color: #005cc5;
  font-weight: 700;
}

.markdown-body .pl-mi {
  color: #24292e;
  font-style: italic;
}

.markdown-body .pl-mb {
  color: #24292e;
  font-weight: 700;
}

.markdown-body .pl-md {
  background-color: #ffeef0;
  color: #b31d28;
}

.markdown-body .pl-mi1 {
  background-color: #f0fff4;
  color: #22863a;
}

.markdown-body .pl-mc {
  background-color: #ffebda;
  color: #e36209;
}

.markdown-body .pl-mi2 {
  background-color: #005cc5;
  color: #f6f8fa;
}

.markdown-body .pl-mdr {
  color: #6f42c1;
  font-weight: 700;
}

.markdown-body .pl-ba {
  color: #586069;
}

.markdown-body .pl-sg {
  color: #959da5;
}

.markdown-body .pl-corl {
  color: #032f62;
  text-decoration: underline;
}

.markdown-body details {
  display: block;
}

.markdown-body summary {
  display: list-item;
}

.markdown-body a {
  background-color: transparent;
}

.markdown-body a:active,
.markdown-body a:hover {
  outline-width: 0;
}

.markdown-body strong {
  font-weight: inherit;
  font-weight: bolder;
}

.markdown-body h1 {
  font-size: 2em;
  margin: .67em 0;
}

.markdown-body img {
  border-style: none;
}

.markdown-body code,
.markdown-body kbd,
.markdown-body pre {
  font-family: monospace,monospace;
  font-size: 1em;
}

.markdown-body hr {
  box-sizing: content-box;
  height: 0;
  overflow: visible;
}

.markdown-body input {
  font: inherit;
  margin: 0;
}

.markdown-body input {
  overflow: visible;
}

.markdown-body [type=checkbox] {
  box-sizing: border-box;
  padding: 0;
}

.markdown-body * {
  box-sizing: border-box;
}

.markdown-body input {
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
}

.markdown-body a {
  color: #0366d6;
  text-decoration: none;
}

.markdown-body a:hover {
  text-decoration: underline;
}

.markdown-body strong {
  font-weight: 600;
}

.markdown-body hr {
  background: transparent;
  border: 0;
  border-bottom: 1px solid #dfe2e5;
  height: 0;
  margin: 15px 0;
  overflow: hidden;
}

.markdown-body hr:before {
  content: "";
  display: table;
}

.markdown-body hr:after {
  clear: both;
  content: "";
  display: table;
}

.markdown-body table {
  border-collapse: collapse;
  border-spacing: 0;
}

.markdown-body td,
.markdown-body th {
  padding: 0;
}

.markdown-body details summary {
  cursor: pointer;
}

.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  margin-bottom: 0;
  margin-top: 0;
}

.markdown-body h1 {
  font-size: 32px;
}

.markdown-body h1,
.markdown-body h2 {
  font-weight: 600;
}

.markdown-body h2 {
  font-size: 24px;
}

.markdown-body h3 {
  font-size: 20px;
}

.markdown-body h3,
.markdown-body h4 {
  font-weight: 600;
}

.markdown-body h4 {
  font-size: 16px;
}

.markdown-body h5 {
  font-size: 14px;
}

.markdown-body h5,
.markdown-body h6 {
  font-weight: 600;
}

.markdown-body h6 {
  font-size: 12px;
}

.markdown-body p {
  margin-bottom: 10px;
  margin-top: 0;
}

.markdown-body blockquote {
  margin: 0;
}

.markdown-body ol,
.markdown-body ul {
  margin-bottom: 0;
  margin-top: 0;
  padding-left: 0;
}

.markdown-body ol ol,
.markdown-body ul ol {
  list-style-type: lower-roman;
}

.markdown-body ol ol ol,
.markdown-body ol ul ol,
.markdown-body ul ol ol,
.markdown-body ul ul ol {
  list-style-type: lower-alpha;
}

.markdown-body dd {
  margin-left: 0;
}

.markdown-body code,
.markdown-body pre {
  font-family: SFMono-Regular,Consolas,Liberation Mono,Menlo,Courier,monospace;
  font-size: 12px;
}

.markdown-body pre {
  margin-bottom: 0;
  margin-top: 0;
}

.markdown-body input::-webkit-inner-spin-button,
.markdown-body input::-webkit-outer-spin-button {
  -webkit-appearance: none;
  appearance: none;
  margin: 0;
}

.markdown-body .border {
  border: 1px solid #e1e4e8!important;
}

.markdown-body .border-0 {
  border: 0!important;
}

.markdown-body .border-bottom {
  border-bottom: 1px solid #e1e4e8!important;
}

.markdown-body .rounded-1 {
  border-radius: 3px!important;
}

.markdown-body .bg-white {
  background-color: #fff!important;
}

.markdown-body .bg-gray-light {
  background-color: #fafbfc!important;
}

.markdown-body .text-gray-light {
  color: #6a737d!important;
}

.markdown-body .mb-0 {
  margin-bottom: 0!important;
}

.markdown-body .my-2 {
  margin-bottom: 8px!important;
  margin-top: 8px!important;
}

.markdown-body .pl-0 {
  padding-left: 0!important;
}

.markdown-body .py-0 {
  padding-bottom: 0!important;
  padding-top: 0!important;
}

.markdown-body .pl-1 {
  padding-left: 4px!important;
}

.markdown-body .pl-2 {
  padding-left: 8px!important;
}

.markdown-body .py-2 {
  padding-bottom: 8px!important;
  padding-top: 8px!important;
}

.markdown-body .pl-3,
.markdown-body .px-3 {
  padding-left: 16px!important;
}

.markdown-body .px-3 {
  padding-right: 16px!important;
}

.markdown-body .pl-4 {
  padding-left: 24px!important;
}

.markdown-body .pl-5 {
  padding-left: 32px!important;
}

.markdown-body .pl-6 {
  padding-left: 40px!important;
}

.markdown-body .f6 {
  font-size: 12px!important;
}

.markdown-body .lh-condensed {
  line-height: 1.25!important;
}

.markdown-body .text-bold {
  font-weight: 600!important;
}

.markdown-body:before {
  content: "";
  display: table;
}

.markdown-body:after {
  clear: both;
  content: "";
  display: table;
}

.markdown-body>:first-child {
  margin-top: 0!important;
}

.markdown-body>:last-child {
  margin-bottom: 0!important;
}

.markdown-body a:not([href]) {
  color: inherit;
  text-decoration: none;
}

.markdown-body blockquote,
.markdown-body dl,
.markdown-body ol,
.markdown-body p,
.markdown-body pre,
.markdown-body table,
.markdown-body ul {
  margin-bottom: 16px;
  margin-top: 0;
}

.markdown-body hr {
  background-color: #e1e4e8;
  border: 0;
  height: .25em;
  margin: 24px 0;
  padding: 0;
}

.markdown-body blockquote {
  border-left: .25em solid #dfe2e5;
  color: #6a737d;
  padding: 0 1em;
}

.markdown-body blockquote>:first-child {
  margin-top: 0;
}

.markdown-body blockquote>:last-child {
  margin-bottom: 0;
}

.markdown-body kbd {
  background-color: #fafbfc;
  border: 1px solid #c6cbd1;
  border-bottom-color: #959da5;
  border-radius: 3px;
  box-shadow: inset 0 -1px 0 #959da5;
  color: #444d56;
  display: inline-block;
  font-size: 11px;
  line-height: 10px;
  padding: 3px 5px;
  vertical-align: middle;
}

.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  font-weight: 600;
  line-height: 1.25;
  margin-bottom: 16px;
  margin-top: 24px;
}

.markdown-body h1 {
  font-size: 2em;
}

.markdown-body h1,
.markdown-body h2 {
  border-bottom: 1px solid #eaecef;
  padding-bottom: .3em;
}

.markdown-body h2 {
  font-size: 1.5em;
}

.markdown-body h3 {
  font-size: 1.25em;
}

.markdown-body h4 {
  font-size: 1em;
}

.markdown-body h5 {
  font-size: .875em;
}

.markdown-body h6 {
  color: #6a737d;
  font-size: .85em;
}

.markdown-body ol,
.markdown-body ul {
  padding-left: 2em;
}

.markdown-body ol ol,
.markdown-body ol ul,
.markdown-body ul ol,
.markdown-body ul ul {
  margin-bottom: 0;
  margin-top: 0;
}

.markdown-body li {
  word-wrap: break-all;
}

.markdown-body li>p {
  margin-top: 16px;
}

.markdown-body li+li {
  margin-top: .25em;
}

.markdown-body dl {
  padding: 0;
}

.markdown-body dl dt {
  font-size: 1em;
  font-style: italic;
  font-weight: 600;
  margin-top: 16px;
  padding: 0;
}

.markdown-body dl dd {
  margin-bottom: 16px;
  padding: 0 16px;
}

.markdown-body table {
  display: block;
  overflow: auto;
  width: 100%;
}

.markdown-body table th {
  font-weight: 600;
}

.markdown-body table td,
.markdown-body table th {
  border: 1px solid #dfe2e5;
  padding: 6px 13px;
}

.markdown-body table tr {
  background-color: #fff;
  border-top: 1px solid #c6cbd1;
}

.markdown-body table tr:nth-child(2n) {
  background-color: #f6f8fa;
}

.markdown-body img {
  background-color: #fff;
  box-sizing: content-box;
  max-width: 100%;
}

.markdown-body img[align=right] {
  padding-left: 20px;
}

.markdown-body img[align=left] {
  padding-right: 20px;
}

.markdown-body code {
  background-color: rgba(27,31,35,.05);
  border-radius: 3px;
  font-size: 85%;
  margin: 0;
  padding: .2em .4em;
}

.markdown-body pre {
  word-wrap: normal;
}

.markdown-body pre>code {
  background: transparent;
  border: 0;
  font-size: 100%;
  margin: 0;
  padding: 0;
  white-space: pre;
  word-break: normal;
}

.markdown-body .highlight {
  margin-bottom: 16px;
}

.markdown-body .highlight pre {
  margin-bottom: 0;
  word-break: normal;
}

.markdown-body .highlight pre,
.markdown-body pre {
  background-color: #f6f8fa;
  border-radius: 3px;
  font-size: 85%;
  line-height: 1.45;
  overflow: auto;
  padding: 16px;
}

.markdown-body pre code {
  background-color: transparent;
  border: 0;
  display: inline;
  line-height: inherit;
  margin: 0;
  max-width: auto;
  overflow: visible;
  padding: 0;
  word-wrap: normal;
}

.markdown-body .commit-tease-sha {
  color: #444d56;
  display: inline-block;
  font-family: SFMono-Regular,Consolas,Liberation Mono,Menlo,Courier,monospace;
  font-size: 90%;
}

.markdown-body .blob-wrapper {
  border-bottom-left-radius: 3px;
  border-bottom-right-radius: 3px;
  overflow-x: auto;
  overflow-y: hidden;
}

.markdown-body .blob-wrapper-embedded {
  max-height: 240px;
  overflow-y: auto;
}

.markdown-body .blob-num {
  -moz-user-select: none;
  -ms-user-select: none;
  -webkit-user-select: none;
  color: rgba(27,31,35,.3);
  cursor: pointer;
  font-family: SFMono-Regular,Consolas,Liberation Mono,Menlo,Courier,monospace;
  font-size: 12px;
  line-height: 20px;
  min-width: 50px;
  padding-left: 10px;
  padding-right: 10px;
  text-align: right;
  user-select: none;
  vertical-align: top;
  white-space: nowrap;
  width: 1%;
}

.markdown-body .blob-num:hover {
  color: rgba(27,31,35,.6);
}

.markdown-body .blob-num:before {
  content: attr(data-line-number);
}

.markdown-body .blob-code {
  line-height: 20px;
  padding-left: 10px;
  padding-right: 10px;
  position: relative;
  vertical-align: top;
}

.markdown-body .blob-code-inner {
  color: #24292e;
  font-family: SFMono-Regular,Consolas,Liberation Mono,Menlo,Courier,monospace;
  font-size: 12px;
  overflow: visible;
  white-space: pre;
  word-wrap: normal;
}

.markdown-body .pl-token.active,
.markdown-body .pl-token:hover {
  background: #ffea7f;
  cursor: pointer;
}

.markdown-body kbd {
  background-color: #fafbfc;
  border: 1px solid #d1d5da;
  border-bottom-color: #c6cbd1;
  border-radius: 3px;
  box-shadow: inset 0 -1px 0 #c6cbd1;
  color: #444d56;
  display: inline-block;
  font: 11px SFMono-Regular,Consolas,Liberation Mono,Menlo,Courier,monospace;
  line-height: 10px;
  padding: 3px 5px;
  vertical-align: middle;
}

.markdown-body :checked+.radio-label {
  border-color: #0366d6;
  position: relative;
  z-index: 1;
}

.markdown-body .tab-size[data-tab-size="1"] {
  -moz-tab-size: 1;
  tab-size: 1;
}

.markdown-body .tab-size[data-tab-size="2"] {
  -moz-tab-size: 2;
  tab-size: 2;
}

.markdown-body .tab-size[data-tab-size="3"] {
  -moz-tab-size: 3;
  tab-size: 3;
}

.markdown-body .tab-size[data-tab-size="4"] {
  -moz-tab-size: 4;
  tab-size: 4;
}

.markdown-body .tab-size[data-tab-size="5"] {
  -moz-tab-size: 5;
  tab-size: 5;
}

.markdown-body .tab-size[data-tab-size="6"] {
  -moz-tab-size: 6;
  tab-size: 6;
}

.markdown-body .tab-size[data-tab-size="7"] {
  -moz-tab-size: 7;
  tab-size: 7;
}

.markdown-body .tab-size[data-tab-size="8"] {
  -moz-tab-size: 8;
  tab-size: 8;
}

.markdown-body .tab-size[data-tab-size="9"] {
  -moz-tab-size: 9;
  tab-size: 9;
}

.markdown-body .tab-size[data-tab-size="10"] {
  -moz-tab-size: 10;
  tab-size: 10;
}

.markdown-body .tab-size[data-tab-size="11"] {
  -moz-tab-size: 11;
  tab-size: 11;
}

.markdown-body .tab-size[data-tab-size="12"] {
  -moz-tab-size: 12;
  tab-size: 12;
}

.markdown-body .task-list-item {
  list-style-type: none;
}

.markdown-body .task-list-item+.task-list-item {
  margin-top: 3px;
}

.markdown-body .task-list-item input {
  margin: 0 .2em .25em -1.6em;
  vertical-align: middle;
}

.markdown-body hr {
  border-bottom-color: #eee;
}

.markdown-body .pl-0 {
  padding-left: 0!important;
}

.markdown-body .pl-1 {
  padding-left: 4px!important;
}

.markdown-body .pl-2 {
  padding-left: 8px!important;
}

.markdown-body .pl-3 {
  padding-left: 16px!important;
}

.markdown-body .pl-4 {
  padding-left: 24px!important;
}

.markdown-body .pl-5 {
  padding-left: 32px!important;
}

.markdown-body .pl-6 {
  padding-left: 40px!important;
}

.markdown-body .pl-7 {
  padding-left: 48px!important;
}

.markdown-body .pl-8 {
  padding-left: 64px!important;
}

.markdown-body .pl-9 {
  padding-left: 80px!important;
}

.markdown-body .pl-10 {
  padding-left: 96px!important;
}

.markdown-body .pl-11 {
  padding-left: 112px!important;
}

.markdown-body .pl-12 {
  padding-left: 128px!important;
}
"""

if __name__ == "__main__":
  main()
