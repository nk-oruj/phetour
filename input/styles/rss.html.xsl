<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:import href="html.xsl"/>
    <xsl:output method="xml" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>

    <xsl:template match="/publication">
        <rss-content>
            <title><xsl:apply-templates select="post|tag" mode="title"/></title>
            <description><xsl:apply-templates select="post|tag" mode="publication"/></description>
        </rss-content>
    </xsl:template>

    <xsl:template match="post|tag" mode="title">
        <xsl:choose>
            <xsl:when test="@status = 'Created'">Created: <xsl:value-of select="@title"/></xsl:when>
            <xsl:when test="@status = 'Revised'">Revised: <xsl:value-of select="@title"/></xsl:when>
        </xsl:choose>
    </xsl:template>

    <xsl:template match="post|tag" mode="publication">
        <xsl:apply-templates select="body/*[not(position() = 1 and self::bold)]"/>
    </xsl:template>
</xsl:stylesheet>
