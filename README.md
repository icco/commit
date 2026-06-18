# commit
Weird messages for Tristan

`GET /` returns a random message as JSON: `{"message": "..."}`.

Pass `?name=<name>` to address the message to someone other than the default
(Tristan), e.g. `GET /?name=Roshan`. Messages are `text/template` strings; a
`{{.Name}}` placeholder is filled with the supplied name. Messages without the
placeholder are returned unchanged.
