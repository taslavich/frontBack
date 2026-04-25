export type Json =
  | string
  | number
  | boolean
  | null
  | { [key: string]: Json | undefined }
  | Json[]

export type Database = {
  // Allows to automatically instantiate createClient with right options
  // instead of createClient<Database, { PostgrestVersion: 'XX' }>(URL, KEY)
  __InternalSupabase: {
    PostgrestVersion: "14.4"
  }
  public: {
    Tables: {
      campaigns: {
        Row: {
          banner_size: string | null
          brand_name: string | null
          budget: number
          clicks: number
          created_at: string
          ctr: number
          daily_budget: number | null
          description: string | null
          end_date: string | null
          even_spend: boolean
          format: string
          format_key: string
          id: string
          impressions: number
          name: string
          price_value: number
          pricing_model: Database["public"]["Enums"]["pricing_model"]
          spent: number
          start_date: string | null
          status: Database["public"]["Enums"]["campaign_status"]
          targeting: Json
          traffic_quality: Database["public"]["Enums"]["traffic_quality"]
          traffic_type: Database["public"]["Enums"]["traffic_type"]
          updated_at: string
          user_id: string
          verticals: string[]
        }
        Insert: {
          banner_size?: string | null
          brand_name?: string | null
          budget?: number
          clicks?: number
          created_at?: string
          ctr?: number
          daily_budget?: number | null
          description?: string | null
          end_date?: string | null
          even_spend?: boolean
          format: string
          format_key: string
          id?: string
          impressions?: number
          name: string
          price_value?: number
          pricing_model?: Database["public"]["Enums"]["pricing_model"]
          spent?: number
          start_date?: string | null
          status?: Database["public"]["Enums"]["campaign_status"]
          targeting?: Json
          traffic_quality?: Database["public"]["Enums"]["traffic_quality"]
          traffic_type?: Database["public"]["Enums"]["traffic_type"]
          updated_at?: string
          user_id: string
          verticals?: string[]
        }
        Update: {
          banner_size?: string | null
          brand_name?: string | null
          budget?: number
          clicks?: number
          created_at?: string
          ctr?: number
          daily_budget?: number | null
          description?: string | null
          end_date?: string | null
          even_spend?: boolean
          format?: string
          format_key?: string
          id?: string
          impressions?: number
          name?: string
          price_value?: number
          pricing_model?: Database["public"]["Enums"]["pricing_model"]
          spent?: number
          start_date?: string | null
          status?: Database["public"]["Enums"]["campaign_status"]
          targeting?: Json
          traffic_quality?: Database["public"]["Enums"]["traffic_quality"]
          traffic_type?: Database["public"]["Enums"]["traffic_type"]
          updated_at?: string
          user_id?: string
          verticals?: string[]
        }
        Relationships: []
      }
      creatives: {
        Row: {
          campaign_id: string
          created_at: string
          description: string | null
          id: string
          image_file_name: string | null
          image_url: string | null
          name: string | null
          storage_path: string | null
          title: string | null
          url: string
        }
        Insert: {
          campaign_id: string
          created_at?: string
          description?: string | null
          id?: string
          image_file_name?: string | null
          image_url?: string | null
          name?: string | null
          storage_path?: string | null
          title?: string | null
          url: string
        }
        Update: {
          campaign_id?: string
          created_at?: string
          description?: string | null
          id?: string
          image_file_name?: string | null
          image_url?: string | null
          name?: string | null
          storage_path?: string | null
          title?: string | null
          url?: string
        }
        Relationships: [
          {
            foreignKeyName: "creatives_campaign_id_fkey"
            columns: ["campaign_id"]
            isOneToOne: false
            referencedRelation: "campaigns"
            referencedColumns: ["id"]
          },
        ]
      }
      profiles: {
        Row: {
          balance: number
          balance_threshold: number
          created_at: string
          email: string | null
          full_name: string | null
          id: string
          manager_telegram: string
          notify_campaign_status: boolean
          notify_low_balance: boolean
          telegram: string | null
          timezone: string | null
          updated_at: string
        }
        Insert: {
          balance?: number
          balance_threshold?: number
          created_at?: string
          email?: string | null
          full_name?: string | null
          id: string
          manager_telegram: string
          notify_campaign_status?: boolean
          notify_low_balance?: boolean
          telegram?: string | null
          timezone?: string | null
          updated_at?: string
        }
        Update: {
          balance?: number
          balance_threshold?: number
          created_at?: string
          email?: string | null
          full_name?: string | null
          id?: string
          manager_telegram?: string
          notify_campaign_status?: boolean
          notify_low_balance?: boolean
          telegram?: string | null
          timezone?: string | null
          updated_at?: string
        }
        Relationships: []
      }
      promo_codes: {
        Row: {
          bonus_percent: number
          code: string
          created_at: string
          current_uses: number
          expires_at: string | null
          id: string
          is_active: boolean
          max_uses: number | null
        }
        Insert: {
          bonus_percent?: number
          code: string
          created_at?: string
          current_uses?: number
          expires_at?: string | null
          id?: string
          is_active?: boolean
          max_uses?: number | null
        }
        Update: {
          bonus_percent?: number
          code?: string
          created_at?: string
          current_uses?: number
          expires_at?: string | null
          id?: string
          is_active?: boolean
          max_uses?: number | null
        }
        Relationships: []
      }
      promo_usage: {
        Row: {
          created_at: string
          id: string
          promo_code_id: string
          topup_request_id: string | null
          user_id: string
        }
        Insert: {
          created_at?: string
          id?: string
          promo_code_id: string
          topup_request_id?: string | null
          user_id: string
        }
        Update: {
          created_at?: string
          id?: string
          promo_code_id?: string
          topup_request_id?: string | null
          user_id?: string
        }
        Relationships: [
          {
            foreignKeyName: "promo_usage_promo_code_id_fkey"
            columns: ["promo_code_id"]
            isOneToOne: false
            referencedRelation: "promo_codes"
            referencedColumns: ["id"]
          },
          {
            foreignKeyName: "promo_usage_topup_request_id_fkey"
            columns: ["topup_request_id"]
            isOneToOne: false
            referencedRelation: "topup_requests"
            referencedColumns: ["id"]
          },
        ]
      }
      topup_requests: {
        Row: {
          amount: number
          bonus_percent: number | null
          created_at: string
          id: string
          payment_method: string
          processed_at: string | null
          processed_by: string | null
          promo_code: string | null
          status: Database["public"]["Enums"]["topup_status"]
          tx_hash: string | null
          user_id: string
        }
        Insert: {
          amount: number
          bonus_percent?: number | null
          created_at?: string
          id?: string
          payment_method: string
          processed_at?: string | null
          processed_by?: string | null
          promo_code?: string | null
          status?: Database["public"]["Enums"]["topup_status"]
          tx_hash?: string | null
          user_id: string
        }
        Update: {
          amount?: number
          bonus_percent?: number | null
          created_at?: string
          id?: string
          payment_method?: string
          processed_at?: string | null
          processed_by?: string | null
          promo_code?: string | null
          status?: Database["public"]["Enums"]["topup_status"]
          tx_hash?: string | null
          user_id?: string
        }
        Relationships: []
      }
      transactions: {
        Row: {
          amount: number
          created_at: string
          description: string | null
          id: string
          reference_id: string | null
          type: Database["public"]["Enums"]["transaction_type"]
          user_id: string
        }
        Insert: {
          amount: number
          created_at?: string
          description?: string | null
          id?: string
          reference_id?: string | null
          type: Database["public"]["Enums"]["transaction_type"]
          user_id: string
        }
        Update: {
          amount?: number
          created_at?: string
          description?: string | null
          id?: string
          reference_id?: string | null
          type?: Database["public"]["Enums"]["transaction_type"]
          user_id?: string
        }
        Relationships: []
      }
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      [_ in never]: never
    }
    Enums: {
      campaign_status:
        | "active"
        | "paused"
        | "draft"
        | "completed"
        | "moderation"
      pricing_model: "cpm" | "cpc"
      topup_status: "pending" | "approved" | "rejected"
      traffic_quality: "common" | "high" | "ultra"
      traffic_type: "mainstream" | "adult" | "mixed"
      transaction_type: "topup" | "spend" | "promo_bonus" | "refund"
    }
    CompositeTypes: {
      [_ in never]: never
    }
  }
}

type DatabaseWithoutInternals = Omit<Database, "__InternalSupabase">

type DefaultSchema = DatabaseWithoutInternals[Extract<keyof Database, "public">]

export type Tables<
  DefaultSchemaTableNameOrOptions extends
    | keyof (DefaultSchema["Tables"] & DefaultSchema["Views"])
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof (DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
        DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Views"])
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? (DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
      DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Views"])[TableName] extends {
      Row: infer R
    }
    ? R
    : never
  : DefaultSchemaTableNameOrOptions extends keyof (DefaultSchema["Tables"] &
        DefaultSchema["Views"])
    ? (DefaultSchema["Tables"] &
        DefaultSchema["Views"])[DefaultSchemaTableNameOrOptions] extends {
        Row: infer R
      }
      ? R
      : never
    : never

export type TablesInsert<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Insert: infer I
    }
    ? I
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Insert: infer I
      }
      ? I
      : never
    : never

export type TablesUpdate<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof DatabaseWithoutInternals },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Update: infer U
    }
    ? U
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Update: infer U
      }
      ? U
      : never
    : never

export type Enums<
  DefaultSchemaEnumNameOrOptions extends
    | keyof DefaultSchema["Enums"]
    | { schema: keyof DatabaseWithoutInternals },
  EnumName extends DefaultSchemaEnumNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"]
    : never = never,
> = DefaultSchemaEnumNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"][EnumName]
  : DefaultSchemaEnumNameOrOptions extends keyof DefaultSchema["Enums"]
    ? DefaultSchema["Enums"][DefaultSchemaEnumNameOrOptions]
    : never

export type CompositeTypes<
  PublicCompositeTypeNameOrOptions extends
    | keyof DefaultSchema["CompositeTypes"]
    | { schema: keyof DatabaseWithoutInternals },
  CompositeTypeName extends PublicCompositeTypeNameOrOptions extends {
    schema: keyof DatabaseWithoutInternals
  }
    ? keyof DatabaseWithoutInternals[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"]
    : never = never,
> = PublicCompositeTypeNameOrOptions extends {
  schema: keyof DatabaseWithoutInternals
}
  ? DatabaseWithoutInternals[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"][CompositeTypeName]
  : PublicCompositeTypeNameOrOptions extends keyof DefaultSchema["CompositeTypes"]
    ? DefaultSchema["CompositeTypes"][PublicCompositeTypeNameOrOptions]
    : never

export const Constants = {
  public: {
    Enums: {
      campaign_status: ["active", "paused", "draft", "completed", "moderation"],
      pricing_model: ["cpm", "cpc"],
      topup_status: ["pending", "approved", "rejected"],
      traffic_quality: ["common", "high", "ultra"],
      traffic_type: ["mainstream", "adult", "mixed"],
      transaction_type: ["topup", "spend", "promo_bonus", "refund"],
    },
  },
} as const
