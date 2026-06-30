import express from "express";
import cors from "cors";
import swaggerUi from "swagger-ui-express";
import { config } from "./config";
import { swaggerSpec } from "./swagger";
import { getAllProviderNames } from "./providers";
import priceRoutes from "./routes/priceRoutes";
import packageRoutes from "./routes/packageRoutes";
import mappingRoutes from "./routes/admin/mappingRoutes";
import hotelRoutes from "./routes/admin/hotelRoutes";
import { closeBrowser } from "./services/puppeteer/manager";
import { startProxySync, stopProxySync } from "./services/proxyService";

const app = express();

app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Swagger UI
app.use("/docs", swaggerUi.serve, swaggerUi.setup(swaggerSpec));

// Routes
app.use("/api/prices", priceRoutes);
app.use("/api/packages", packageRoutes);
app.use("/api/admin/mappings", mappingRoutes);
app.use("/api/admin/hotels", hotelRoutes);

// Health check
app.get("/health", (_req, res) => {
  res.json({ status: "ok", providers: getAllProviderNames() });
});

// Graceful shutdown
process.on("SIGTERM", async () => {
  stopProxySync();
  await closeBrowser();
  process.exit(0);
});

app.listen(config.port, async () => {
  console.log(`PriceBot ${config.port} portunda calisiyor`);
  console.log(`Providers: ${getAllProviderNames().join(", ")}`);
  console.log(`Swagger UI: http://localhost:${config.port}/docs`);

  // Start proxy rotation sync
  await startProxySync();
});
