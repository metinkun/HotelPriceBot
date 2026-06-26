-- CreateTable
CREATE TABLE "hotel_mappings" (
    "id" SERIAL NOT NULL,
    "provider" TEXT NOT NULL,
    "internal_hotel_id" TEXT NOT NULL,
    "provider_hotel_id" TEXT NOT NULL,
    "hotel_name" TEXT,
    "metadata" JSONB,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "hotel_mappings_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "hotel_mappings_provider_provider_hotel_id_idx" ON "hotel_mappings"("provider", "provider_hotel_id");

-- CreateIndex
CREATE INDEX "hotel_mappings_provider_is_active_idx" ON "hotel_mappings"("provider", "is_active");

-- CreateIndex
CREATE UNIQUE INDEX "hotel_mappings_provider_internal_hotel_id_key" ON "hotel_mappings"("provider", "internal_hotel_id");
