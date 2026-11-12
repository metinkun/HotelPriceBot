// Admin API istemcisi: JWT localStorage'da, her istege Authorization: Bearer eklenir.
import { createApi } from "@kit";
export const { api, getToken, setToken, download } = createApi("pricebot_admin_token");
export { parseReq, fmtDate, fmtDay, fmtTL, fmtRel, WEEKDAYS, WEEKDAYS_SHORT, addDays, today } from "@kit";
