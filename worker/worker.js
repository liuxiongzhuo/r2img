// src/index.ts
var src_default = {
    async fetch(request, env, ctx) {
      const authHeader = request.headers.get("Authorization");
      if (authHeader !== "Bearer " + env.AUTH_API_KEY) {
        return new Response("Invalid API key", { status: 401 });
      }
      const url = new URL(request.url);
      const path = url.pathname;
      if (path === "/upload") {
        if (request.method !== "POST") {
          return new Response("Method Not Allowed", { status: 405 });
        }
        const contentType = request.headers.get("Content-Type");
        if (!contentType || contentType.indexOf("multipart/form-data") === -1) {
          return new Response("Bad Request", { status: 400 });
        }
        try {
          const formData = await request.formData();
          const file = formData.get("file");
          if (!(file instanceof File)) {
            return new Response("No file uploaded", { status: 400 });
          }
          const options = {
            httpMetadata: {
              contentType: file.type
              // 设置文件的内容类型
            }
          };
          await env.R2_BUCKET.put(file.name, file.stream(), options);
          return new Response(JSON.stringify({ message: "File uploaded successfully" }), {
            headers: { "Content-Type": "application/json" },
            status: 200
          });
        } catch (error) {
          return new Response("Failed to upload file", { status: 500 });
        }
      } else if (path.startsWith("/i/")) {
        const parts = path.split("/");
        if (parts.length < 3) {
          return new Response("Not Found", { status: 404 });
        }
        const fileName = parts[2];
        const object = await env.R2_BUCKET.get(fileName);
        if (!object) {
          return new Response("Not Found", { status: 404 });
        }
        return new Response(object.body, {
          headers: {
            "Content-Type": object.httpMetadata?.contentType || "application/octet-stream"
          }
        });
      } else {
        return new Response("Not Found", { status: 404 });
      }
    }
  };
  export {
    src_default as default
  };
  //# sourceMappingURL=index.js.map
  