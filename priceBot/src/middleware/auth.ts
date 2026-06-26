import { Request, Response, NextFunction } from "express";
import { config } from "../config";

export function adminAuth(req: Request, res: Response, next: NextFunction) {
  if (!config.adminApiKey) {
    res.status(500).json({ error: "ADMIN_API_KEY ayarlanmamis." });
    return;
  }

  const apiKey = req.headers["x-api-key"];
  if (!apiKey || apiKey !== config.adminApiKey) {
    res.status(401).json({ error: "Gecersiz API anahtari" });
    return;
  }

  next();
}
