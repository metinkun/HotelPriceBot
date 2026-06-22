import express from "express";
import cors from "cors";
import swaggerUi from "swagger-ui-express";
import { config } from "./config";
import { swaggerSpec } from "./swagger";
import priceRoutes from "./routes/priceRoutes";
import hotelMappingRoutes from "./routes/hotelMappingRoutes";

const app = express();

app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Swagger UI
app.use("/docs", swaggerUi.serve, swaggerUi.setup(swaggerSpec));

// Routes
app.use("/api/prices", priceRoutes);
app.use("/api/admin/mappings", hotelMappingRoutes);

// Health check
app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

app.listen(config.port, () => {
  console.log(`Server ${config.port} portunda calisiyor`);
  console.log(`Swagger UI: http://localhost:${config.port}/docs`);
});
