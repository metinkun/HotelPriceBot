-- CreateTable
CREATE TABLE "proxies" (
    "id" SERIAL NOT NULL,
    "webshare_id" TEXT NOT NULL,
    "proxy_address" TEXT NOT NULL,
    "port" INTEGER NOT NULL,
    "username" TEXT NOT NULL,
    "password" TEXT NOT NULL,
    "country_code" TEXT,
    "valid" BOOLEAN NOT NULL DEFAULT true,
    "last_verification" TIMESTAMP(3),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "proxies_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "proxies_webshare_id_key" ON "proxies"("webshare_id");

-- CreateIndex
CREATE INDEX "proxies_valid_idx" ON "proxies"("valid");
