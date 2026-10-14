# Stable Popunder SDK URL

This incremental cabinet patch only adds optional PUBLISHER_PUBLIC_SCRIPT_URL.
Empty (default) keeps the old PUBLISHER_PUBLIC_URL + /static/adManager.js value.
A configured HTTPS script URL is returned unchanged as public_script_url by
GET /api/publisher/ad-codes. Set it to the stable SDK URL deployed by pub, e.g.
https://STABLE_DOMAIN/sdk.js. Runtime domain rotation/filter processing stays in pub.
The existing frontend should use the returned URL and keep data-ad-code-id unchanged.
No frontend source was supplied. This does not change Code fields, PATCH, Redis sync,
CH statistics, the 30-second revenue sync, ledger or payouts.

Enable pub anti-block and validate its infrastructure first, then set this ENV and
restart cabinet to issue new snippets. To disable issuance, clear the ENV and
restart cabinet. Already published SDK snippets remain supported by pub's disabled
SDK fallback; do not delete their stable endpoint. Full deployment/ENV/Nginx/tests
and original-file coverage are in RTB docs/publisher/ANTIBLOCK.md and README_APPLY.
