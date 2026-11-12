// Portal API istemcisi: JWT localStorage'da, her istege Authorization: Bearer eklenir.
import { createApi } from "@kit";
export const { api, getToken, setToken, download } = createApi("pricebot_portal_token");
export { parseReq, fmtDate, fmtTL, fmtRel, WEEKDAYS, WEEKDAYS_SHORT, addDays, today } from "@kit";
