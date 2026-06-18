# commit
Weird messages for Tristan

`GET /` returns a random message as JSON: `{"message": "..."}`.

Pass `?name=<name>` to address the message to someone other than the default
(Tristan), e.g. `GET /?name=Roshan`. Messages that don't address anyone by name
are returned unchanged.
