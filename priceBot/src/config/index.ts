import dotenv from "dotenv";
dotenv.config();

export const config = {
  port: parseInt(process.env.PORT || "3000", 10),
  adminApiKey: process.env.ADMIN_API_KEY || "",

  providers: {
    etstur: {
      cookie: process.env.ETSTUR_COOKIE || "",
    },
    tatilsepeti: {
      cookie: process.env.TATILSEPETI_COOKIE || "",
      verificationToken: process.env.TATILSEPETI_VERIFICATION_TOKEN || "",
    },
  },

  proxy: {
    host: process.env.PROXY_HOST || "",
    port: parseInt(process.env.PROXY_PORT || "0", 10),
    username: process.env.PROXY_USERNAME || "",
    password: process.env.PROXY_PASSWORD || "",
  },

  webshare: {
    apiKey: process.env.WEBSHARE_API_KEY || "",
    syncIntervalMs: parseInt(process.env.PROXY_SYNC_INTERVAL_MS || "300000", 10),
  },

  puppeteer: {
    headless: process.env.PUPPETEER_HEADLESS !== "false",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || undefined,
  },
};
