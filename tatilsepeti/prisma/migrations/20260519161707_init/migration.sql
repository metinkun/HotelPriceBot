-- CreateTable
CREATE TABLE "hotel_mappings" (
    "id" SERIAL NOT NULL,
    "internal_hotel_id" TEXT NOT NULL,
    "tatilsepeti_hotel_id" TEXT NOT NULL,
    "hotel_name" TEXT,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "hotel_mappings_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "hotel_mappings_tatilsepeti_hotel_id_idx" ON "hotel_mappings"("tatilsepeti_hotel_id");

-- CreateIndex
CREATE UNIQUE INDEX "hotel_mappings_internal_hotel_id_key" ON "hotel_mappings"("internal_hotel_id");
