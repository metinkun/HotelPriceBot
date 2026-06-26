import express from "express";
import cors from "cors";
import swaggerUi from "swagger-ui-express";
import { config } from "./config";
import { swaggerSpec } from "./swagger";
import { getAllProviderNames } from "./providers";
import priceRoutes from "./routes/priceRoutes";
import packageRoutes from "./routes/packageRoutes";
import mappingRoutes from "./routes/admin/mappingRoutes";
import { closeBrowser } from "./services/puppeteer/manager";

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

// Health check
app.get("/health", (_req, res) => {
  res.json({ status: "ok", providers: getAllProviderNames() });
});

// Graceful shutdown
process.on("SIGTERM", async () => {
  await closeBrowser();
  process.exit(0);
});

app.listen(config.port, () => {
  console.log(`PriceBot ${config.port} portunda calisiyor`);
  console.log(`Providers: ${getAllProviderNames().join(", ")}`);
  console.log(`Swagger UI: http://localhost:${config.port}/docs`);
});
