import { Request, Response, NextFunction } from "express";
import { config } from "../config";

export function adminAuth(req: Request, res: Response, next: NextFunction) {
  const apiKey = req.headers["x-api-key"];

  if (!config.adminApiKey) {
    res.status(500).json({ error: "ADMIN_API_KEY yapilandirilmamis" });
    return;
  }

  if (apiKey !== config.adminApiKey) {
    res.status(401).json({ error: "Gecersiz API anahtari" });
    return;
  }

  next();
}
