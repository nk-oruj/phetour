<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xml" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>

    <xsl:template match="/rss-item">
        <xsl:if test="@type != 'post' and @type != 'tag'">
            <xsl:message terminate="yes">Unknown RSS item type</xsl:message>
        </xsl:if>
        <rss-content>
            <title><xsl:value-of select="@id"/> - <xsl:value-of select="document/meta/title/@value"/></title>
            <description>
                <xsl:apply-templates select="document/body/*"/>
            </description>
        </rss-content>
    </xsl:template>

    <xsl:template match="bold[not(preceding-sibling::*)]"/>

    <xsl:template match="bold">
        <p><strong><xsl:value-of select="normalize-space(.)"/></strong></p>
        <xsl:text>&#10;</xsl:text>
    </xsl:template>

    <xsl:template match="text">
        <xsl:variable name="content" select="normalize-space(.)"/>
        <xsl:if test="$content != ''">
            <p><xsl:value-of select="$content"/></p>
            <xsl:text>&#10;</xsl:text>
        </xsl:if>
    </xsl:template>

    <xsl:template match="link">
        <p>
            <a>
                <xsl:attribute name="href">
                    <xsl:call-template name="absolute-href">
                        <xsl:with-param name="href" select="@href"/>
                    </xsl:call-template>
                </xsl:attribute>
                <xsl:value-of select="."/>
            </a>
        </p>
        <xsl:text>&#10;</xsl:text>
    </xsl:template>

    <xsl:template match="item[not(preceding-sibling::*[1][self::item])]">
        <ul>
            <xsl:apply-templates select=". | following-sibling::*[
                self::item and
                generate-id(preceding-sibling::*[not(self::item)][1]) =
                generate-id(current()/preceding-sibling::*[not(self::item)][1])
            ]" mode="item-group"/>
        </ul>
        <xsl:text>&#10;</xsl:text>
    </xsl:template>

    <xsl:template match="item"/>

    <xsl:template match="item" mode="item-group">
        <li><xsl:value-of select="normalize-space(.)"/></li>
    </xsl:template>

    <xsl:template match="code"/>

    <xsl:template name="absolute-href">
        <xsl:param name="href"/>
        <xsl:choose>
            <xsl:when test="starts-with($href, 'https://') or starts-with($href, 'http://')">
                <xsl:value-of select="$href"/>
            </xsl:when>
            <xsl:otherwise><xsl:value-of select="concat(/rss-item/@site-url, $href)"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>
</xsl:stylesheet>
