# Creative API v2

Creative image upload is separated from creative CRUD. MinIO remains private and
all permanent image URLs are served by the backend.

All JSON responses use the existing envelope:

```json
{
  "success": true,
  "errorMsg": "",
  "data": {}
}
```

## 1. Upload an image

```http
POST /api/campaigns/{campaignID}/creative-images
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

Form fields:

- `file` — required image file;
- `filename` — optional display filename.

Allowed actual MIME types: JPEG, PNG, GIF and WebP. Maximum size: 64 MiB.
Popunder campaigns cannot upload images.

Response data:

```json
{
  "image_id": "uuid",
  "campaign_id": "uuid",
  "image_url": "https://twinbid.io/api/media/uuid",
  "filename": "banner.png",
  "mime_type": "image/png",
  "file_format": "png",
  "size_bytes": 12345,
  "created_at": "2026-07-25T00:00:00Z",
  "updated_at": "2026-07-25T00:00:00Z"
}
```

An uploaded but unused image is retained indefinitely.

## 2. Create a creative

```http
POST /api/campaigns/{campaignID}/creatives
Authorization: Bearer <token>
Content-Type: application/json
```

Example for an image banner:

```json
{
  "creative_name": "Banner 300x250",
  "adm": "<a href=\"https://example.com\"><img src=\"https://twinbid.io/api/media/IMAGE_ID\" width=\"300\" height=\"250\"></a>",
  "banner_type": "img",
  "image_id": "IMAGE_ID",
  "trackers_macros": {},
  "w": 300,
  "h": 250
}
```

Example for an iframe/HTML banner:

```json
{
  "creative_name": "Iframe 300x250",
  "adm": "<iframe src=\"https://example.com/ad\" width=\"300\" height=\"250\"></iframe>",
  "banner_type": "iframe",
  "trackers_macros": {},
  "w": 300,
  "h": 250
}
```

## 3. Patch a creative

```http
PATCH /api/creatives/{id}
Authorization: Bearer <token>
Content-Type: application/json
```

Only supplied fields are changed. Image semantics:

- omit `image_id` to preserve the current image;
- send another UUID to replace the image;
- send `"image_id": null` to remove it;
- switching a banner to `banner_type: "iframe"` automatically removes the old image.

The old S3 object and image row are removed after a successful replacement.

## 4. Delete a creative

```http
DELETE /api/creatives/{id}
```

The creative, its private MinIO object and its `creative_images` row are removed.
Campaign deletion also removes all campaign image objects.

## 5. Read media

```http
GET /api/media/{imageID}
HEAD /api/media/{imageID}
```

These routes require no bearer token because their URLs are embedded in ADM.
They expose only one UUID-addressed file through the backend. MinIO credentials,
bucket listing, upload and deletion are not exposed.

## Validation matrix

| Campaign format | banner_type | image_id |
|---|---|---|
| banner image | `img`, required | required |
| banner iframe/HTML | `iframe`, required | forbidden |
| native | omitted | required |
| push | omitted | required |
| popunder | omitted | forbidden |

`adm` and `creative_name` are always required. Banner `w` and `h` are required.
Native and push `title` and `description` are required.
