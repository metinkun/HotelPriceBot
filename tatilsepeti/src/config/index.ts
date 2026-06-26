import dotenv from "dotenv";
dotenv.config();

export const config = {
  port: parseInt(process.env.PORT || "3000", 10),
  adminApiKey: process.env.ADMIN_API_KEY || "",
  tatilsepetiCookie: process.env.TATILSEPETI_COOKIE || "",
};
