# Public Publisher script URL

GET /api/publisher/ad-codes returns `public_script_url`. The default is
`PUBLISHER_PUBLIC_URL` + `/static/pageHelper.js`. Set the public origin to
`https://carloverrcvbg.site`. An explicit HTTPS `PUBLISHER_PUBLIC_SCRIPT_URL`
takes priority; update or remove an old override before restarting the cabinet.

The cabinet frontend uses the returned URL and emits the server's `short_code`
in the `food` attribute:

```html
<script async src="https://carloverrcvbg.site/static/pageHelper.js" food="tb_EXISTING_CODE"></script>
```

For anti-block routing use the deployed stable `/sdk.js` as the explicit URL.
Verify the Publisher routes on the public hostname before publishing the
cabinet frontend. See RTB docs/publisher/ANTIBLOCK.md for SDK and Nginx setup.
