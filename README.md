# subnautica-alerts

A small Go CLI that speaks text in a Subnautica PDA-style voice. It fetches
TTS audio from the Streamlabs Polly proxy, applies a modulation chain to give
the voice its characteristic "PDA" timbre, and plays it through the speakers.

```
go build -o pda.exe
```

Prebuilt `pda.exe` binaries are also produced by the
[Build workflow](../../actions/workflows/build.yml) — grab the `pda` artifact
from the latest successful run.

## Usage

Speak a single line and exit:

```
pda.exe "All systems nominal"
```

Listen mode — start an HTTP server on `:8787` and speak anything POSTed to it:

```
pda.exe
```

```
curl -X POST --data "Detecting multiple leviathan class lifeforms" http://localhost:8787/tts
```

## Hooking it into the Streamlabs alert box

`custom.js` is a browser snippet that watches the Streamlabs Alert Box widget.
Whenever an alert finishes playing, it grabs the alert's message text and POSTs
it to the local `pda` server (`http://127.0.0.1:8787/tts`), so every alert is
read out in the PDA voice.

### 1. Run `pda` in listen mode

Start it with no arguments and leave it running while you stream:

```
pda.exe
```

The custom code talks to `127.0.0.1:8787`, so `pda` must run on the **same
machine** as Streamlabs (Streamlabs Desktop's alert box browser source runs
locally, so this works out of the box).

### 2. Open the Alert Box custom code editor

The Alert Box widget supports custom HTML/CSS/JS. Where you find it depends on
which Streamlabs you use:

- **Streamlabs Desktop**: *Editor* tab → click the **Alert Box** widget →
  **gear / settings** icon → scroll to the bottom of the settings panel and
  enable **Custom Code** (the `{ }` toggle).
- **streamlabs.com dashboard**: *Alert Box* settings page → scroll to the
  bottom → toggle **Enable Custom HTML/CSS** (sometimes labelled **Custom
  Code**).

Either way you get a code editor with **HTML / CSS / JS / Fields** tabs.

### 3. Paste the script into the JS tab

1. Select the **JS** tab.
2. Paste the entire contents of [`custom.js`](custom.js) into it. Leave the
   HTML and CSS tabs untouched — the script does not need them.
3. Click **Save**.

That's it. The next time an alert fires, once it finishes the message text is
sent to your running `pda` instance and spoken in the PDA voice. The snippet is
safe to re-save: it cleans up its previous instance before re-installing, so
editing and saving repeatedly won't stack duplicate handlers.

### Troubleshooting

- **Nothing is spoken**: confirm `pda.exe` is running with no arguments and
  that POSTing to `http://localhost:8787/tts` (see the curl example above)
  works on its own.
- **Works from curl but not from Streamlabs**: the alert box source must be on
  the same machine as `pda`. The script uses a `no-cors` request, so no server
  CORS configuration is required.
- **Changed the port?** Update `POST_URL` at the top of `custom.js` to match;
  the server's `:8787` is currently fixed in `server.go`.
